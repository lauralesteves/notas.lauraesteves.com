# Infra do frontend (CloudFormation). O backend tem o próprio template SAM em backend/.
#
#   cp .env.example .env.prod   # preencha CERTIFICATE_ARN e HOSTED_ZONE_ID
#   make validate
#   make frontend

ENV_FILE ?= .env.prod
-include $(ENV_FILE)

STACK_NAME ?= notas-lauraesteves-frontend
AWS_REGION ?= us-east-1
DOMAIN_NAME ?= notas.lauraesteves.com
API_DOMAIN_NAME ?= api.notas.lauraesteves.com
WS_DOMAIN_NAME ?= ws.notas.lauraesteves.com

export AWS_PAGER :=

.PHONY: frontend validate outputs check-env

check-env:
	@test -f $(ENV_FILE) || (echo "Faltando $(ENV_FILE) (copie de .env.example)"; exit 1)
	@test -n "$(CERTIFICATE_ARN)" || (echo "CERTIFICATE_ARN vazio em $(ENV_FILE)"; exit 1)
	@test -n "$(HOSTED_ZONE_ID)" || (echo "HOSTED_ZONE_ID vazio em $(ENV_FILE)"; exit 1)

## Valida o template (somente leitura)
validate:
	aws cloudformation validate-template \
		--region $(AWS_REGION) \
		--template-body file://frontend.yml

## Cria/atualiza bucket, CloudFront, Function e DNS
frontend: check-env
	aws cloudformation deploy \
		--region $(AWS_REGION) \
		--stack-name $(STACK_NAME) \
		--template-file frontend.yml \
		--no-fail-on-empty-changeset \
		--parameter-overrides \
			DomainName=$(DOMAIN_NAME) \
			ApiDomainName=$(API_DOMAIN_NAME) \
			WebSocketDomainName=$(WS_DOMAIN_NAME) \
			CertificateArn=$(CERTIFICATE_ARN) \
			HostedZoneId=$(HOSTED_ZONE_ID)
	$(MAKE) outputs

## Mostra os outputs (DistributionId vira o secret CLOUDFRONT_DISTRIBUTION_ID)
outputs:
	aws cloudformation describe-stacks \
		--region $(AWS_REGION) \
		--stack-name $(STACK_NAME) \
		--query "Stacks[0].Outputs" \
		--output table
