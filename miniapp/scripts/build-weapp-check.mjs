import { spawn } from 'node:child_process'

const child = spawn(process.execPath, ['node_modules/@tarojs/cli/bin/taro', 'build', '--type', 'weapp'], {
  env: process.env,
  stdio: ['inherit', 'pipe', 'pipe'],
})
let output = ''
for (const stream of [child.stdout, child.stderr]) {
  stream.on('data', (chunk) => {
    output += chunk.toString()
    process.stdout.write(chunk)
  })
}
child.on('error', (error) => {
  console.error(error)
  process.exitCode = 1
})
child.on('close', (code) => {
  const plain = output.replace(/\x1b\[[0-9;]*m/g, '')
  const warning = /\b(?:warnings?|\w+Warning)\b|⚠|警告/i.test(plain)
  if (warning) console.error('Mini program build warnings must be resolved.')
  process.exitCode = code === 0 && !warning ? 0 : 1
})
