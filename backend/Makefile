APP_NAME=notas.lauraesteves.com

.PHONY: help dev build clean deploy validate test test-ci fmt lint preview-email

help:
	@echo "Available targets:"
	@echo "  make dev            - Run the local dev server on :8080 (in-memory, no AWS)"
	@echo "  make build          - sam build"
	@echo "  make deploy         - sam deploy with parameters from .env.prod"
	@echo "  make validate       - sam validate --lint"
	@echo "  make test           - Run all tests"
	@echo "  make fmt            - Format all Go files"
	@echo "  make lint           - go vet"
	@echo "  make preview-email  - Render the weekly report into .preview/"

dev:
	go run ./cmd/devserver

clean:
	rm -rf .aws-sam

build: clean
	sam build

validate:
	sam validate --lint

deploy: build
	@set -a && \
	source .env.prod && \
	set +a && \
	sam deploy \
		--no-confirm-changeset \
		--parameter-overrides \
		HostedZoneId=$$HOSTED_ZONE_ID \
		CertificateArn=$$CERTIFICATE_ARN \
		MailFrom=$$MAIL_FROM \
		ReportTo=$$REPORT_TO \
		--capabilities CAPABILITY_IAM

TEST_FLAGS ?= -cover -count=1 -timeout=60s

test:
	go test $(TEST_FLAGS) ./...

test-ci:
	go test -race $(TEST_FLAGS) ./...

fmt:
	gofmt -s -w .

lint:
	go vet ./...

preview-email:
	go run ./cmd/preview-email -out .preview
	@echo "Abra .preview/weekly_report.html no navegador."
