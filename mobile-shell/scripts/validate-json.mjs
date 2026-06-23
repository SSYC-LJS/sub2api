import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const jsonFiles = ['package.json', 'manifest.json', 'pages.json', 'tsconfig.json']
const requiredFiles = ['index.html', 'main.ts', 'App.vue', 'pages/index/index.vue']

for (const file of jsonFiles) {
  const fullPath = resolve(process.cwd(), file)
  JSON.parse(readFileSync(fullPath, 'utf8'))
  console.log(`ok ${file}`)
}

for (const file of requiredFiles) {
  const fullPath = resolve(process.cwd(), file)
  readFileSync(fullPath, 'utf8')
  console.log(`ok ${file}`)
}
