.PHONY: init \
       frontend-server frontend-build frontend-test frontend-deploy \
       backend-dev backend-build backend-test backend-deploy backend-preview-email \
       infra-frontend

## Setup
init:
	cp scripts/pre-push .git/hooks/pre-push
	chmod +x .git/hooks/pre-push
	@echo "Git hooks instalados."

## Frontend
frontend-server:
	$(MAKE) -C frontend server

frontend-build:
	$(MAKE) -C frontend build

frontend-test:
	$(MAKE) -C frontend test

frontend-deploy:
	$(MAKE) -C frontend deploy

## Backend
backend-dev:
	$(MAKE) -C backend dev

backend-build:
	$(MAKE) -C backend build

backend-test:
	$(MAKE) -C backend test

backend-deploy:
	$(MAKE) -C backend deploy

backend-preview-email:
	$(MAKE) -C backend preview-email

## Infra
infra-frontend:
	$(MAKE) -C infra frontend
