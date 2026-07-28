export default {
  "nav": {
    "ranking": "排行榜",
    "modelMarket": "模型广场",
    "requestResponseLogs": "请求采集"
  },
  "dashboard": {
    "tokenRankingTitle": "排行榜",
    "tokenRankingSubtitle": "按今日、本周、本月统计用户 Token 用量 Top 10",
    "tokenRankingEmpty": "暂无排行榜数据",
    "rankingPlaceholder": "虚席以待",
    "rankingFirstDescription": "显卡杀手",
    "rankingSecondDescription": "风扇狂转",
    "rankingThirdDescription": "CPU 报警",
    "rankingToday": "今日",
    "rankingWeek": "本周",
    "rankingMonth": "本月"
  },
  "usageRanking": {
    "title": "排行榜",
    "description": "查看今日、本周、本月用户 Token 用量 Top 10"
  },
  "channelStatus": {
    "requestWindows": {
      "title": "请求情况",
      "requests": "请求",
      "success": "成功率",
      "errors": "失败率",
      "errorRate": "错误率",
      "avgHourlyRequests": "平均每小时",
      "level": {
        "idle": "空闲",
        "normal": "正常",
        "busy": "繁忙",
        "congested": "拥挤"
      }
    }
  },
  "modelMarket": {
    "title": "模型广场",
    "description": "按您当前可用分组展示可用模型。卡片顶部显示分组名称、当前倍率和管理员配置的推荐指数。",
    "badge": "按可用分组自动生成",
    "searchPlaceholder": "搜索模型、分组或供应商...",
    "allProviders": "全部供应商",
    "availableModels": "当前分组可用模型",
    "emptyTitle": "暂无可用模型",
    "emptyDescription": "当前没有匹配的分组或模型，或管理员尚未为您的可用分组配置模型。",
    "exclusive": "专属分组",
    "public": "公开分组",
    "modelCount": "{count} 个模型",
    "pricing": {
      "openHint": "单击查看模型价格参考",
      "title": "{group} 价格参考",
      "titleFallback": "价格参考",
      "rateHint": "以下价格已按当前分组倍率折算，仅供参考。",
      "noPricing": "未配置定价",
      "billingModeToken": "按 Token",
      "billingModePerRequest": "按次",
      "billingModeImage": "按图片",
      "inputPrice": "输入",
      "outputPrice": "输出",
      "cacheWritePrice": "缓存写入",
      "cacheReadPrice": "缓存读取",
      "imageOutputPrice": "图片输出",
      "perRequestPrice": "每次请求",
      "perImagePrice": "单张价格",
      "intervals": "阶梯价格",
      "unitPerMillion": "/ 1M token",
      "unitPerRequest": "/ 次",
      "unitPerImage": "/ 张"
    },
    "stats": {
      "models": "可用模型",
      "providers": "供应商",
      "groups": "可用分组"
    },
    "recommendation": {
      "normal": "正常",
      "moderate": "适中",
      "recommended": "推荐",
      "super": "超级性价比"
    }
  },
  "availableChannels": {
    "pricing": {
      "perImagePrice": "单张价格",
      "unitPerImage": "/ 张"
    }
  },
  "redeem": {
    "buyActivationCode": "购买激活码"
  },
  "admin": {
    "requestResponseLogs": {
      "title": "请求/返回采集",
      "description": "查看用户与站点之间的完整双向请求和返回数据"
    },
    "groups": {
      "recommendation": {
        "label": "模型广场推荐字样",
        "labelPlaceholder": "例如：超值推荐 / 稳定优选",
        "labelHint": "留空时按倍率自动显示默认字样。",
        "stars": "模型广场星级",
        "starsHint": "范围 3-5 星，最低保留 3 星；星级越高卡片推荐配色越明显。"
      }
    },
    "accounts": {
      "openai": {
        "codexImageGenerationBridge": "Codex 图片生成桥接",
        "codexImageGenerationBridgeDesc": "账号级策略优先于渠道和全局配置。仅控制 Codex 走 /responses 文本端点时是否注入 image_generation 工具；不影响独立图片生成接口。",
        "codexImageGenerationBridgeInherit": "跟随渠道",
        "codexImageGenerationBridgeInheritDesc": "不写入账号覆盖，继续使用渠道或全局策略。",
        "codexImageGenerationBridgeEnabled": "强制开启",
        "codexImageGenerationBridgeEnabledDesc": "允许 Codex /responses 请求获得图片工具注入。",
        "codexImageGenerationBridgeDisabled": "强制关闭",
        "codexImageGenerationBridgeDisabledDesc": "阻断 Codex /responses 的图片工具注入。",
        "codexImageGenerationBridgeBadgeInherit": "渠道策略",
        "codexImageGenerationBridgeBadgeEnabled": "账号开启",
        "codexImageGenerationBridgeBadgeDisabled": "账号关闭"
      }
    },
    "settings": {
      "tabs": {
        "notifications": "通知/Webhook"
      },
      "webhook": {
        "title": "系统通知 Webhook",
        "description": "配置系统事件通知 Webhook。当前会在兑换码兑换成功等事件触发后异步推送，不用于支付回调。",
        "enable": "启用 Webhook 通知",
        "enableHint": "开启后，兑换码兑换成功会发送通知到配置的 Webhook 地址。",
        "url": "Webhook 地址",
        "urlHint": "支持飞书/Lark 自定义机器人地址，也支持接收 JSON 的普通 HTTP(S) 地址。",
        "format": "消息格式",
        "timeout": "请求超时（秒）",
        "bearerToken": "Bearer Token（可选）",
        "bearerTokenPlaceholder": "可选，用于 Authorization: Bearer ...",
        "bearerTokenConfiguredPlaceholder": "********",
        "bearerTokenHint": "如果目标服务不需要鉴权可以留空。",
        "bearerTokenConfiguredHint": "密钥已配置，留空以保留当前值。",
        "eventHint": "说明：这不是支付回调；它监听系统事件。兑换码兑换成功事件名为 redeem_code.used，payload 会包含兑换码类型、面值、有效期、用户等信息。"
      },
      "site": {
        "contactQrCodeUrl": "联系我们二维码",
        "contactQrCodeUrlPlaceholder": "https://example.com/contact-qrcode.png 或 data:image/...",
        "contactQrCodeUrlHint": "兼容旧字段：将作为“联系站长”二维码使用。",
        "contactWebmasterQrCode": "联系站长二维码",
        "contactWebmasterQrCodeHint": "上传本地 PNG/JPG 后会保存为 data:image，点击侧边栏“联系我们”后展示在“联系站长”位置。",
        "contactGroupQrCode": "加入群聊二维码",
        "contactGroupQrCodeHint": "上传本地 PNG/JPG 后会保存为 data:image，点击侧边栏“联系我们”后展示在“加入群聊”位置。",
        "activationCodePurchaseUrl": "激活码购买地址",
        "activationCodePurchaseUrlPlaceholder": "https://example.com/buy-code",
        "activationCodePurchaseUrlHint": "兑换页面的购买入口地址。留空则不显示购买入口。"
      }
    }
  }
} as const
