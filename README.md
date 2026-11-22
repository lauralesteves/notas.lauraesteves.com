# notas.lauraesteves.com

Bloco de notas colaborativo sem login, inspirado no [dontpad](https://dontpad.com). A URL é a nota: `notas.lauraesteves.com/qualquer-coisa`.

- Salva sozinho quando você para de digitar.
- Quem estiver com a mesma URL aberta vê as mudanças em tempo real.
- Subpáginas: `notas.lauraesteves.com/faculdade/calculo`.
- Pelo terminal: `curl notas.lauraesteves.com/raw/qualquer-coisa`.
- As notas somem 15 dias depois da última edição.
- Toda nota é pública para quem souber a URL. Não guarde senhas aqui.

## Stack

| Parte | Tecnologia |
|-|-|
| Frontend | React 19, Vite, Tailwind v4, TypeScript, bun, Biome |
| Backend | Go, AWS Lambda (arm64, `provided.al2023`), SAM |
| Dados | DynamoDB on-demand, TTL nativo para expiração |
| Tempo real | API Gateway WebSocket API |
| E-mail | SES v2 (relatório semanal) |
| Hospedagem | S3 + CloudFront, DNS no Route53 |

## Estrutura

```
frontend/   app React (S3 + CloudFront)
backend/    Lambdas em Go, template SAM e servidor local
infra/      CloudFormation do frontend (bucket, CloudFront, DNS)
docs/       contrato da API (docs/api.md)
scripts/    git hooks
```

## Desenvolvimento

```bash
make init              # instala o pre-push hook
make backend-dev       # API + WebSocket em memória em :8080
make frontend-server   # Vite em :5173, apontando para :8080
```

Testes: `make backend-test` e `make frontend-test`.

Prévia do e-mail semanal: `make backend-preview-email`.

## Deploy

1. Certificado ACM em us-east-1 para `notas.lauraesteves.com` e `*.notas.lauraesteves.com`, validado por DNS no Route53.
2. `make backend-deploy` (APIs, Lambdas, DynamoDB, domínios `api.notas` e `ws.notas`).
3. `make infra-frontend` (bucket, CloudFront, registro `notas`).
4. `make frontend-deploy`.

Depois do primeiro deploy, os workflows em `.github/workflows/` fazem o deploy de cada parte no push para `main`.

## Contrato

O formato das chamadas HTTP e das mensagens do WebSocket está em [docs/api.md](docs/api.md).
