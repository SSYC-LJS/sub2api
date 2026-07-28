package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sysutil"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SystemHandler handles system-related operations
type SystemHandler struct {
	updateSvc  systemUpdateService
	lockSvc    *service.SystemOperationLockService
	updateJobs *systemUpdateJobStore
}

type systemUpdateJobStore struct {
	mu   sync.RWMutex
	jobs map[string]*systemUpdateJob
}

type systemUpdateJob struct {
	OperationID     string    `json:"operation_id"`
	Status          string    `json:"status"`
	Message         string    `json:"message"`
	NeedRestart     bool      `json:"need_restart"`
	AlreadyUpToDate bool      `json:"already_up_to_date"`
	CurrentVersion  string    `json:"current_version,omitempty"`
	LatestVersion   string    `json:"latest_version,omitempty"`
	Error           string    `json:"error,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at,omitempty"`
}

// systemUpdateTimeout bounds a full in-place update or rollback: the release
// manifest fetch plus a large binary download over slow links. It must stay
// above the GitHub download client timeout (10 minutes) so the download owns
// its own deadline.
const systemUpdateTimeout = 15 * time.Minute

// systemUpdateContext detaches a long-running update/rollback from the HTTP
// request lifetime. Browsers and reverse proxies commonly abort idle requests
// after 30-60s (axios default, nginx proxy_read_timeout), which canceled
// c.Request.Context() mid-download and killed the update with
// "download failed: context canceled" (#4504). The swap keeps running after a
// client disconnect; a later retry then hits the system operation lock or
// reports "Already up to date".
func systemUpdateContext(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(base, systemUpdateTimeout)
}

type systemUpdateService interface {
	CheckUpdate(ctx context.Context, force bool) (*service.UpdateInfo, error)
	PerformUpdate(ctx context.Context) error
	ListRollbackVersions(ctx context.Context) ([]service.RollbackVersion, error)
	RollbackToVersion(ctx context.Context, version string) error
	Rollback() error
}

// NewSystemHandler creates a new SystemHandler
func NewSystemHandler(updateSvc systemUpdateService, lockSvc *service.SystemOperationLockService) *SystemHandler {
	return &SystemHandler{
		updateSvc:  updateSvc,
		lockSvc:    lockSvc,
		updateJobs: newSystemUpdateJobStore(),
	}
}

// GetVersion returns the current version
// GET /api/v1/admin/system/version
func (h *SystemHandler) GetVersion(c *gin.Context) {
	info, _ := h.updateSvc.CheckUpdate(c.Request.Context(), false)
	response.Success(c, gin.H{
		"version": info.CurrentVersion,
	})
}

// CheckUpdates checks for available updates
// GET /api/v1/admin/system/check-updates
func (h *SystemHandler) CheckUpdates(c *gin.Context) {
	force := c.Query("force") == "true"
	info, err := h.updateSvc.CheckUpdate(c.Request.Context(), force)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, info)
}

// PerformUpdate downloads and applies the update
// POST /api/v1/admin/system/update
func (h *SystemHandler) PerformUpdate(c *gin.Context) {
	operationID := buildSystemOperationID(c, "update")
	payload := gin.H{"operation_id": operationID}
	executeAdminIdempotentJSON(c, "admin.system.update", payload, service.DefaultSystemOperationIdempotencyTTL(), func(ctx context.Context) (any, error) {
		lock, release, err := h.acquireSystemLock(ctx, operationID)
		if err != nil {
			return nil, err
		}
		job := h.updateJobs.start(lock.OperationID())
		go h.runUpdateJob(lock, release, job.OperationID)
		return gin.H{
			"message":      "Update started in background",
			"status":       job.Status,
			"operation_id": job.OperationID,
		}, nil
	})
}

// GetUpdateStatus returns background update job status
// GET /api/v1/admin/system/update/status/:operation_id
func (h *SystemHandler) GetUpdateStatus(c *gin.Context) {
	operationID := strings.TrimSpace(c.Param("operation_id"))
	if operationID == "" {
		operationID = strings.TrimSpace(c.Query("operation_id"))
	}
	if operationID == "" {
		response.Error(c, http.StatusBadRequest, "operation_id is required")
		return
	}
	job, ok := h.updateJobs.get(operationID)
	if !ok {
		response.Error(c, http.StatusNotFound, "update operation not found")
		return
	}
	response.Success(c, job)
}

func (h *SystemHandler) runUpdateJob(lock *service.SystemOperationLock, release func(string, bool), operationID string) {
	ctx := context.Background()
	releaseReason := ""
	succeeded := false
	defer func() {
		release(releaseReason, succeeded)
	}()

	if err := h.updateSvc.PerformUpdate(ctx); err != nil {
		if errors.Is(err, service.ErrNoUpdateAvailable) {
			info, checkErr := h.updateSvc.CheckUpdate(ctx, false)
			if checkErr != nil {
				releaseReason = "SYSTEM_UPDATE_FAILED"
				h.updateJobs.fail(operationID, checkErr)
				return
			}
			succeeded = true
			h.updateJobs.completeAlreadyUpToDate(operationID, info)
			return
		}
		releaseReason = "SYSTEM_UPDATE_FAILED"
		h.updateJobs.fail(operationID, err)
		return
	}
	succeeded = true
	h.updateJobs.completeNeedRestart(operationID)
}

// GetRollbackVersions lists versions available for rollback.
// GET /api/v1/admin/system/rollback-versions
func (h *SystemHandler) GetRollbackVersions(c *gin.Context) {
	versions, err := h.updateSvc.ListRollbackVersions(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{
		"versions": versions,
	})
}

// Rollback restores a previous version. An empty version keeps the legacy
// behavior of restoring the local backup binary.
// POST /api/v1/admin/system/rollback
func (h *SystemHandler) Rollback(c *gin.Context) {
	var req struct {
		Version string `json:"version"`
	}
	if c.Request.Body != nil && c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	targetVersion := strings.TrimSpace(req.Version)

	operation := "rollback"
	if targetVersion != "" {
		operation += ":" + targetVersion
	}
	operationID := buildSystemOperationID(c, operation)
	payload := gin.H{"operation_id": operationID, "version": targetVersion}
	executeAdminIdempotentJSON(c, "admin.system.rollback", payload, service.DefaultSystemOperationIdempotencyTTL(), func(ctx context.Context) (any, error) {
		lock, release, err := h.acquireSystemLock(ctx, operationID)
		if err != nil {
			return nil, err
		}
		var releaseReason string
		succeeded := false
		defer func() {
			release(releaseReason, succeeded)
		}()

		if targetVersion != "" {
			rollbackCtx, cancel := systemUpdateContext(ctx)
			defer cancel()
			err = h.updateSvc.RollbackToVersion(rollbackCtx, targetVersion)
		} else {
			err = h.updateSvc.Rollback()
		}
		if err != nil {
			releaseReason = "SYSTEM_ROLLBACK_FAILED"
			return nil, err
		}
		succeeded = true

		return gin.H{
			"message":      "Rollback completed. Please restart the service.",
			"need_restart": true,
			"version":      targetVersion,
			"operation_id": lock.OperationID(),
		}, nil
	})
}

// RestartService restarts the systemd service
// POST /api/v1/admin/system/restart
func (h *SystemHandler) RestartService(c *gin.Context) {
	operationID := buildSystemOperationID(c, "restart")
	payload := gin.H{"operation_id": operationID}
	executeAdminIdempotentJSON(c, "admin.system.restart", payload, service.DefaultSystemOperationIdempotencyTTL(), func(ctx context.Context) (any, error) {
		lock, release, err := h.acquireSystemLock(ctx, operationID)
		if err != nil {
			return nil, err
		}
		succeeded := false
		defer func() {
			release("", succeeded)
		}()

		// Schedule service restart in background after sending response
		// This ensures the client receives the success response before the service restarts
		go func() {
			// Wait a moment to ensure the response is sent
			time.Sleep(500 * time.Millisecond)
			sysutil.RestartServiceAsync()
		}()
		succeeded = true
		return gin.H{
			"message":      "Service restart initiated",
			"operation_id": lock.OperationID(),
		}, nil
	})
}

func newSystemUpdateJobStore() *systemUpdateJobStore {
	return &systemUpdateJobStore{jobs: make(map[string]*systemUpdateJob)}
}

func (s *systemUpdateJobStore) start(operationID string) *systemUpdateJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := &systemUpdateJob{
		OperationID: operationID,
		Status:      "running",
		Message:     "Update is running in background",
		StartedAt:   time.Now(),
	}
	s.jobs[operationID] = job
	return cloneSystemUpdateJob(job)
}

func (s *systemUpdateJobStore) get(operationID string) (*systemUpdateJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[operationID]
	if !ok {
		return nil, false
	}
	return cloneSystemUpdateJob(job), true
}

func (s *systemUpdateJobStore) fail(operationID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.ensure(operationID)
	job.Status = "failed"
	job.Message = "Update failed"
	job.Error = err.Error()
	job.FinishedAt = time.Now()
}

func (s *systemUpdateJobStore) completeNeedRestart(operationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.ensure(operationID)
	job.Status = "completed"
	job.Message = "Update completed. Please restart the service."
	job.NeedRestart = true
	job.FinishedAt = time.Now()
}

func (s *systemUpdateJobStore) completeAlreadyUpToDate(operationID string, info *service.UpdateInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.ensure(operationID)
	job.Status = "completed"
	job.Message = "Already up to date"
	job.AlreadyUpToDate = true
	if info != nil {
		job.CurrentVersion = info.CurrentVersion
		job.LatestVersion = info.LatestVersion
	}
	job.FinishedAt = time.Now()
}

func (s *systemUpdateJobStore) ensure(operationID string) *systemUpdateJob {
	job, ok := s.jobs[operationID]
	if !ok {
		job = &systemUpdateJob{OperationID: operationID, StartedAt: time.Now()}
		s.jobs[operationID] = job
	}
	return job
}

func cloneSystemUpdateJob(job *systemUpdateJob) *systemUpdateJob {
	if job == nil {
		return nil
	}
	clone := *job
	return &clone
}

func (h *SystemHandler) acquireSystemLock(
	ctx context.Context,
	operationID string,
) (*service.SystemOperationLock, func(string, bool), error) {
	if h.lockSvc == nil {
		return nil, nil, service.ErrIdempotencyStoreUnavail
	}
	lock, err := h.lockSvc.Acquire(ctx, operationID)
	if err != nil {
		return nil, nil, err
	}
	release := func(reason string, succeeded bool) {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = h.lockSvc.Release(releaseCtx, lock, succeeded, reason)
	}
	return lock, release, nil
}

func buildSystemOperationID(c *gin.Context, operation string) string {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		return "sysop-" + operation + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	actorScope := "admin:0"
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		actorScope = "admin:" + strconv.FormatInt(subject.UserID, 10)
	}
	seed := operation + "|" + actorScope + "|" + c.FullPath() + "|" + key
	hash := service.HashIdempotencyKey(seed)
	if len(hash) > 24 {
		hash = hash[:24]
	}
	return "sysop-" + hash
}
