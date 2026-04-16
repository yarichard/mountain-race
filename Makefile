build:
	docker build -t mountain-race .

run:
	docker run --env-file .env -p 8003:8003 mountain-race

local-build:
	cd frontend && npm ci && npm run build
	rm -rf backend/static
	cp -r frontend/out backend/static
	cd backend && go build -o server .

local-run:
	cd backend && ./server

.PHONY: build run local-build local-run
