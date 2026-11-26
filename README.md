# notas.lauraesteves.com

Bloco de notas colaborativo sem login, inspirado no [dontpad](https://dontpad.com). A URL é a nota: `notas.lauraesteves.com/qualquer-coisa`.

- Salva sozinho quando você para de digitar.
- Quem estiver com a mesma URL aberta vê as mudanças em tempo real.
- Subpáginas: `notas.lauraesteves.com/faculdade/calculo`.
- Pelo terminal: `curl notas.lauraesteves.com/raw/qualquer-coisa`, ou instale os comandos `nota` e `nota-add` com `curl -fsSL https://notas.lauraesteves.com/install.sh | sh` (bash ou zsh).
- As notas somem 15 dias depois da última edição.
- Toda nota é pública para quem souber a URL. Não guarde senhas aqui.

## Sumário

- [Stack](#stack)
- [Arquitetura](#arquitetura)
  - [Visão geral](#visão-geral)
  - [Por que serverless](#por-que-serverless)
  - [Salvar e propagar uma edição](#salvar-e-propagar-uma-edição)
  - [Ciclo de vida do WebSocket](#ciclo-de-vida-do-websocket)
  - [Modelo de dados](#modelo-de-dados)
  - [Camadas do backend](#camadas-do-backend)
  - [Sincronização no navegador](#sincronização-no-navegador)
  - [Roteamento no CloudFront](#roteamento-no-cloudfront)
  - [Relatório semanal](#relatório-semanal)
  - [Segurança e limites](#segurança-e-limites)
- [Estrutura](#estrutura)
- [Desenvolvimento](#desenvolvimento)
- [CI/CD](#cicd)
- [Deploy](#deploy)
- [Contrato](#contrato)

## Stack

| Parte | Tecnologia |
|-|-|
| Frontend | React 19, Vite, Tailwind v4, TypeScript, bun, Biome, Vitest |
| Backend | Go, AWS Lambda (arm64, `provided.al2023`), SAM |
| Dados | DynamoDB on-demand, TTL nativo para expiração |
| Tempo real | API Gateway WebSocket API |
| E-mail | SES v2 (relatório semanal), agendado pelo EventBridge |
| Hospedagem | S3 + CloudFront (OAC, HTTP/3), DNS no Route53, certificado ACM |
| CI/CD | GitHub Actions |

## Arquitetura

### Visão geral

Tudo roda em serviços gerenciados da AWS, em `us-east-1`. Não existe servidor ligado esperando requisição: o site é estático no CloudFront, cada rota da API é uma Lambda pequena em Go e o estado fica todo no DynamoDB.

```mermaid
flowchart LR
    user(["Navegador"])
    cli(["curl / terminal"])

    subgraph edge["Borda"]
        r53["Route53"]
        cf["CloudFront<br/>+ CloudFront Function<br/>(SPA rewrite)"]
    end

    subgraph static["Site estático"]
        s3[("S3<br/>bucket privado<br/>via OAC")]
    end

    subgraph api["API (api.notas)"]
        http["API Gateway<br/>HTTP API"]
        lget["λ notes-get"]
        lsave["λ notes-save"]
        lchild["λ children-get"]
        lrawget["λ raw-get"]
        lrawsave["λ raw-save"]
    end

    subgraph rt["Tempo real (ws.notas)"]
        ws["API Gateway<br/>WebSocket API"]
        lconn["λ ws-connect"]
        ldisc["λ ws-disconnect"]
        ldef["λ ws-default"]
    end

    subgraph data["Dados"]
        notes[("DynamoDB<br/>notas-notes")]
        conns[("DynamoDB<br/>notas-connections")]
    end

    subgraph jobs["Jobs"]
        cron["EventBridge<br/>seg 9h BRT"]
        lrep["λ weekly-report"]
        ses["SES"]
    end

    user --> r53 --> cf
    cli --> cf
    cf -- "/*" --> s3
    cf -- "/raw/*" --> http
    user -- "fetch" --> http
    user <-- "wss" --> ws

    http --> lget & lsave & lchild & lrawget & lrawsave
    ws --> lconn & ldisc & ldef

    lget & lchild & lrawget --> notes
    lsave & lrawsave --> notes
    lsave & lrawsave --> conns
    lsave & lrawsave -. "@connections POST" .-> ws
    lconn & ldisc & ldef --> conns
    lconn & ldisc & ldef -. "presence" .-> ws

    cron --> lrep --> notes
    lrep --> ses -. "e-mail" .-> owner(["Dona do site"])

    classDef aws fill:#232f3e,stroke:#ff9900,color:#fff
    classDef lambda fill:#ff9900,stroke:#c46200,color:#000
    classDef db fill:#3b48cc,stroke:#1f2a8a,color:#fff
    classDef actor fill:#f4f4f5,stroke:#71717a,color:#111
    class r53,cf,http,ws,cron,ses aws
    class lget,lsave,lchild,lrawget,lrawsave,lconn,ldisc,ldef,lrep lambda
    class notes,conns,s3 db
    class user,cli,owner actor
```

Três domínios, um certificado ACM para `notas.lauraesteves.com` e `*.notas.lauraesteves.com`:

| Domínio | Aponta para | Uso |
|-|-|-|
| `notas.lauraesteves.com` | CloudFront | SPA e `/raw/*` para o terminal |
| `api.notas.lauraesteves.com` | HTTP API (regional) | Leitura e escrita de notas |
| `ws.notas.lauraesteves.com` | WebSocket API (regional) | Atualizações em tempo real e presença |

### Por que serverless

O tráfego de um bloco de notas pessoal é baixo e em rajadas: horas parado, depois alguém digitando por dez minutos. Pagar por requisição encaixa melhor do que manter uma instância ligada.

- **Custo perto de zero em repouso.** Lambda, API Gateway e DynamoDB on-demand cobram por uso. CloudFront e S3 custam centavos para um SPA pequeno.
- **Nenhum servidor para manter.** Sem patch de sistema operacional, sem autoscaling para configurar, sem processo para reiniciar.
- **Uma Lambda por rota.** Cada função tem só a permissão IAM de que precisa (a de leitura não consegue escrever) e um binário pequeno.
- **Go em arm64 com runtime `provided.al2023`.** Binário nativo, cold start curto, 128 MB de memória bastam. Graviton é mais barato que x86.
- **Expiração de graça.** O TTL nativo do DynamoDB apaga notas e conexões vencidas sem job de limpeza. Como o TTL pode atrasar, toda leitura também confere `expiresAt` e trata nota vencida como inexistente.
- **WebSocket gerenciado.** O API Gateway segura as conexões abertas; as Lambdas só rodam quando alguém conecta, desconecta, manda `ping` ou quando uma nota muda.

Os trade-offs aceitos: o API Gateway derruba conexões depois de 2h (o cliente reconecta), o índice `byPath` é eventualmente consistente (a presença se corrige no próximo `ping`) e mensagens de WebSocket têm limite de tamanho (notas grandes mandam só um aviso `changed`).

### Salvar e propagar uma edição

O cliente salva com versionamento otimista: manda a versão em que a edição se baseou e o DynamoDB só aceita a escrita se ela ainda for a atual (escrita condicional). Depois de salvar, a mesma Lambda avisa as outras abas abertas na nota.

```mermaid
sequenceDiagram
    autonumber
    actor A as Aba A (digitando)
    participant H as HTTP API
    participant S as λ notes-save
    participant N as DynamoDB notes
    participant C as DynamoDB connections
    participant W as WebSocket API
    actor B as Aba B (lendo)

    Note over A: para de digitar por 1s<br/>(ou 5s digitando sem parar)
    A->>H: PUT /notes/{path}<br/>{body, baseVersion: 7}<br/>X-Connection-Id: A
    H->>S: invoke
    S->>N: GetItem(path)
    alt versão atual != baseVersion
        S-->>A: 409 conflict + nota atual
    else versão confere
        S->>N: PutItem version=8<br/>condição: version = 7
        S->>C: Query byPath(path)
        loop até 10 em paralelo, pulando A
            S->>W: POST @connections/{id}
            W-->>B: {"type":"update", version: 8, body}
        end
        S-->>A: 200 {version: 8, expiresAt}
    end
```

- Corpo até 28.000 bytes vai inteiro na mensagem `update`. Acima disso vai `changed` e quem recebe faz `GET`.
- Corpo vazio apaga a nota e propaga `update` com `version: 0`.
- Conexão que responde `410 Gone` é apagada da tabela na hora.
- `PUT /raw/{path}` (pelo terminal) é o mesmo fluxo com `force: true` (last-write-wins).

### Ciclo de vida do WebSocket

```mermaid
sequenceDiagram
    autonumber
    actor U as Navegador
    participant W as WebSocket API
    participant L as λ ws-*
    participant C as DynamoDB connections

    U->>W: wss://ws.notas...?path=foo
    W->>L: $connect
    L->>C: Query byPath(foo)
    alt já tem 50 conexões
        L-->>U: 429 (cliente cai para polling)
    else ok
        L->>C: PutItem {connectionId, path, ttl: +2h}
        L-->>U: presence para os outros
    end

    U->>W: {"action":"ping"}
    W->>L: $default
    L-->>U: {"type":"pong", connectionId, count}
    Note over U,L: ping a cada 5 min (o gateway<br/>derruba após 10 min ocioso)

    U->>W: fecha a aba
    W->>L: $disconnect
    L->>C: DeleteItem
    L-->>U: presence atualizado para os outros
```

Se o WebSocket não conectar, o cliente faz polling com `GET /notes/{path}?since={version}` a cada 5s, só com a aba visível. Ao reconectar (backoff de 1s até 30s) ele busca a nota de novo para não perder nada.

### Modelo de dados

Duas tabelas DynamoDB on-demand, ambas com TTL.

```mermaid
erDiagram
    NOTES {
        string path PK "faculdade/calculo"
        string parent "faculdade"
        string kind "NOTE (fixo)"
        string body "até 100.000 bytes"
        number version "otimista, começa em 1"
        string createdAt
        string updatedAt
        number expiresAt "TTL: updatedAt + 15 dias"
    }
    CONNECTIONS {
        string connectionId PK
        string path "nota aberta"
        string connectedAt
        number ttl "TTL: +2h"
    }
    NOTES ||--o{ CONNECTIONS : "path"
    NOTES ||--o{ NOTES : "parent"
```

| Tabela | Índice | Chave | Para quê |
|-|-|-|-|
| `notas-notes` | tabela | `path` | Ler e salvar uma nota |
| `notas-notes` | GSI `byParent` | `parent` + `path` | Listar subpáginas (`/children`) |
| `notas-notes` | GSI `byUpdated` | `kind` + `updatedAt` | Notas mais recentes (relatório semanal) |
| `notas-connections` | tabela | `connectionId` | Conectar e desconectar |
| `notas-connections` | GSI `byPath` | `path` | Quem está na nota (broadcast e presença) |

Os GSIs projetam só os atributos necessários (`INCLUDE`) para manter as leituras baratas.

### Camadas do backend

O mesmo código de negócio serve as Lambdas e o servidor local. Só muda a ponta: em produção os adaptadores são DynamoDB e o `@connections` do API Gateway; em desenvolvimento são memória e um hub de WebSocket em Go.

```mermaid
flowchart TB
    subgraph entry["Pontos de entrada"]
        direction LR
        lambdas["lambdas/*/main.go<br/>(uma por rota)"]
        dev["cmd/devserver<br/>(:8080, tudo em memória)"]
        prev["cmd/preview-email"]
    end

    subgraph ctrl["controllers"]
        direction LR
        adapt["LambdaHTTP / LambdaWS<br/>adaptam eventos do API Gateway"]
        hnd["NotesController<br/>WsController"]
    end

    subgraph svc["services"]
        direction LR
        ns["NotesService<br/>validação, versão, conflito"]
        bs["BroadcastService<br/>update / changed / presence"]
        cs["ConnectionsService<br/>connect, ping, limite de 50"]
        rep["report<br/>templates HTML + texto"]
    end

    subgraph ports["repositories e external"]
        direction LR
        dyn["DynamoDB<br/>notes + connections"]
        mem["Memória<br/>(dev e testes)"]
        apigw["apigw.Broadcaster"]
        sesx["ses.Sender (MIME)"]
    end

    lambdas --> adapt --> hnd
    dev --> hnd
    prev --> rep
    hnd --> ns & cs
    ns --> bs
    cs --> bs
    ns & bs & cs --> dyn & mem
    bs --> apigw
    rep --> dyn
    rep --> sesx
```

`helpers` concentra a regra de caminho (segmentos `[a-z0-9_-]`, até 5 níveis, nomes reservados), que o frontend replica em `src/lib/path.ts`.

### Sincronização no navegador

`src/lib/noteSync.ts` é um motor de sincronização sem React, testado com Vitest. `useNote` só o liga ao componente. O status mostrado na barra de baixo segue esta máquina:

```mermaid
stateDiagram-v2
    [*] --> loading
    loading --> saved: GET ok
    saved --> saving: digitou (debounce 1s, teto 5s)
    saving --> saved: 200
    saving --> conflict: 409
    saving --> retrying: 429
    saving --> error: 5xx
    saving --> offline: sem rede
    saving --> too_large: acima de 100 KB
    retrying --> saving: backoff
    error --> saving: backoff
    offline --> saving: rede voltou
    too_large --> saving: texto diminuiu
    conflict --> saved: usar a versão nova
    conflict --> saving: manter a minha (force)
    saved --> conflict: update remoto com edição local pendente
```

Quando chega uma nota do servidor (WebSocket, polling ou o corpo de um `409`), o cliente decide entre ignorar (eco próprio ou versão velha), adotar (o remoto já é igual ao texto local), aplicar (não há edição local pendente) ou abrir o banner de conflito. Ao aplicar texto remoto, a posição do cursor é preservada (`src/lib/cursor.ts`). Saves com falha tentam de novo com backoff exponencial até 30s.

### Roteamento no CloudFront

```mermaid
flowchart LR
    req(["GET notas.lauraesteves.com/..."]) --> raw{"/raw/*?"}
    raw -- sim --> api["HTTP API<br/>sem cache<br/>aceita HTTP e HTTPS"]
    raw -- não --> fn{"último segmento<br/>tem ponto?"}
    fn -- não --> idx["reescreve para /index.html<br/>(a SPA lê a nota)"]
    fn -- sim --> s3["arquivo do S3<br/>(assets, robots.txt...)"]
    s3 -- 403 --> fallback["404 com index.html"]
```

Como caminho de nota nunca tem ponto, a CloudFront Function separa nota de arquivo estático sem consultar nada. `/raw/*` aceita HTTP puro para `curl notas.lauraesteves.com/raw/x` funcionar sem `-L`.

### Relatório semanal

Toda segunda às 9h (BRT), o EventBridge dispara `notas-weekly-report`. Ela consulta o GSI `byUpdated` pelas notas editadas nos últimos 7 dias, monta um e-mail em HTML (tema escuro, que se mantém no Gmail) e em texto puro, e envia pelo SES como "Cortex". `make backend-preview-email` gera o e-mail localmente para conferir o layout.

### Segurança e limites

| Camada | Proteção |
|-|-|
| CloudFront | HSTS, CSP restrita (`connect-src` só para `api.notas` e `ws.notas`), `X-Frame-Options: DENY`, `Permissions-Policy`, TLS 1.2+ |
| S3 | Bucket privado, acesso só pelo CloudFront via OAC, criptografia AES-256 |
| API Gateway | Throttling de 20 req/s (rajada de 40) no HTTP e no WebSocket, CORS só para o domínio e `localhost:5173` |
| Lambda | IAM mínimo por função (leitura não escreve; só quem salva ou gerencia conexões pode postar em `@connections`) |
| Domínio | Corpo até 100 KB, caminho validado no backend, 50 conexões por nota, notas expiram em 15 dias |

Não há autenticação, por design: a URL é o segredo. Daí o aviso de não guardar senhas.

## Estrutura

```
frontend/       app React (S3 + CloudFront)
  src/lib/        lógica pura: api, noteSync, saveScheduler, socket, path, cursor
  src/hooks/      useNote, useNoteSocket, usePageMeta
backend/        Lambdas em Go, template SAM e servidor local
  lambdas/        um main.go por função (api/*, ws/*, workers/*)
  internal/       controllers, services, repositories, external, models, helpers
  cmd/            devserver e preview-email
infra/          CloudFormation do frontend (bucket, CloudFront, DNS)
docs/           contrato da API (docs/api.md)
scripts/        git hooks
```

## Desenvolvimento

```bash
make init              # instala o pre-push hook
make backend-dev       # API + WebSocket em memória em :8080
make frontend-server   # Vite em :5173, apontando para :8080
```

Testes: `make backend-test` e `make frontend-test`.

Prévia do e-mail semanal: `make backend-preview-email`.

O `devserver` implementa o contrato inteiro (HTTP e WebSocket) com os mesmos services das Lambdas, então não precisa de AWS nem de Docker para desenvolver.

## CI/CD

```mermaid
flowchart LR
    push(["git push"]) --> hook["pre-push hook<br/>build + testes<br/>do que mudou"]
    hook --> gh{"GitHub Actions"}

    gh -- "backend/**<br/>qualquer branch" --> bb["Backend Build<br/>go build, vet,<br/>test -race,<br/>sam validate --lint,<br/>sam build"]
    gh -- "frontend/**<br/>em main" --> fb["Frontend Build<br/>bun install,<br/>biome lint,<br/>vitest, vite build"]

    gh -- "backend/**<br/>em main" --> bd["Backend Deploy<br/>go test,<br/>sam build + deploy"]
    gh -- "frontend/**<br/>em main" --> fd["Frontend Deploy<br/>test, build,<br/>s3 sync,<br/>invalidação CloudFront"]

    bd --> aws[("AWS<br/>us-east-1")]
    fd --> aws
```

## Deploy

1. Certificado ACM em us-east-1 para `notas.lauraesteves.com` e `*.notas.lauraesteves.com`, validado por DNS no Route53.
2. `make backend-deploy` (APIs, Lambdas, DynamoDB, domínios `api.notas` e `ws.notas`).
3. `make infra-frontend` (bucket, CloudFront, registro `notas`).
4. `make frontend-deploy`.

Depois do primeiro deploy, os workflows em `.github/workflows/` fazem o deploy de cada parte no push para `main`.

## Contrato

O formato das chamadas HTTP e das mensagens do WebSocket está em [docs/api.md](docs/api.md).
