# Architecture

Este monorepo usa um layout Go idiomatico para multiplos binarios.

## Estrutura

```text
cmd/
  ftm-backend/
  ftm-ftp-server/
internal/
  backend/
    api/
    auth/
    config/
    store/
  ftp/
    config/
    ftpserver/
  files/
  minitoml/
configs/
apps/
  web/
```

## Regras

- `cmd/<binario>` deve ficar pequeno: parse de flags, carregamento de config e inicializacao.
- `internal/backend` pertence ao backend HTTP/API.
- `internal/ftp` pertence ao servidor FTP.
- Codigo compartilhado fica em pacotes pequenos dentro de `internal/`.
- O servidor FTP continua independente do backend.
- O backend controla o FTP como processo separado.
- O frontend web deve conversar somente com o backend, nunca diretamente com o FTP.

## Fluxo

```text
React dashboard
    |
    v
ftm-backend HTTP/API
    |
    v
ftm-ftp-server process
```

O backend le `configs/backend.toml`, persiste dados locais em arquivo binario e usa JWT para autenticar usuarios do painel.

O FTP le `configs/ftp-server.toml`, autentica usuarios FTP e publica status em JSON para o backend/dashboard.
