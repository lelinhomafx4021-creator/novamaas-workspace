.DEFAULT_GOAL := help

.PHONY: help local remote dev-local dev-remote

help:
	@printf '%s\n' 'make local       Build for the local backend (http://127.0.0.1:3000)' 'make remote      Build for the remote backend (https://gateway.ai.shilijia.xyz)' 'make dev-local   Watch and build for the local backend' 'make dev-remote  Watch and build for the remote backend'

local:
	MINIAPP_API_BASE_URL=http://127.0.0.1:3000 bun run build:weapp

remote:
	MINIAPP_API_BASE_URL=https://gateway.ai.shilijia.xyz bun run build:weapp

dev-local:
	MINIAPP_API_BASE_URL=http://127.0.0.1:3000 bun run dev:weapp

dev-remote:
	MINIAPP_API_BASE_URL=https://gateway.ai.shilijia.xyz bun run dev:weapp
