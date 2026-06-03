ECR_REPO = 704496393752.dkr.ecr.eu-west-3.amazonaws.com/mountain-race
ECR_REGION = eu-west-3

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

push-ecr:
	aws ecr get-login-password --region $(ECR_REGION) | docker login --username AWS --password-stdin $(ECR_REPO)
	docker tag mountain-race:latest $(ECR_REPO):latest
	docker push $(ECR_REPO):latest

.PHONY: build run local-build local-run push-ecr
