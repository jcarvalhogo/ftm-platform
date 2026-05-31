# ftm-platform

Monorepo for the FTM FTP server, local control agent, desktop app, and web dashboard.

## Estrutura inicial

```text
apps/
  web/                  Futuro dashboard React
cmd/
  ftm-backend/          Entrada do backend HTTP/API
  ftm-ftp-server/       Entrada do servidor FTP
configs/
  backend.toml          Configuracao do backend
  ftp-server.toml       Configuracao do servidor FTP
docs/
  architecture.md       Decisoes de arquitetura
internal/
  backend/              Implementacao privada do backend
  ftp/                  Implementacao privada do servidor FTP
  files/                Helpers de filesystem
  minitoml/             Parser TOML minimo usado pelos servicos
```

Este layout segue o padrao mais usado em projetos Go com multiplos binarios:

- `cmd/<binario>` contem somente bootstrap, flags e composicao de dependencias.
- `internal/<servico>` contem codigo privado da aplicacao.
- `internal/<util>` contem utilitarios compartilhados dentro do modulo.
- `configs/` guarda exemplos de configuracao.
- `apps/` fica reservado para clientes nao-Go, como o futuro React dashboard.

## Servidor FTP

O servidor FTP tem entrada em `cmd/ftm-ftp-server` e implementacao em `internal/ftp`. Ele ja suporta:

- login por usuario/senha;
- raiz isolada por usuario;
- modo passivo `PASV` e `EPSV`;
- `LIST`, `NLST`, `RETR`, `STOR`, `DELE`, `MKD`, `RMD`, `CWD`, `PWD`, `TYPE`, `SYST`, `FEAT`, `NOOP` e `QUIT`;
- arquivo de PID;
- arquivo JSON de status para o backend/dashboard.

Configuracao padrao:

```text
configs/ftp-server.toml
```

Rodar manualmente:

```bash
go run ./cmd/ftm-ftp-server -config ./configs/ftp-server.toml
```

Build:

```bash
mkdir -p build
go build -o build/ftm-ftp-server ./cmd/ftm-ftp-server
```

Teste rapido:

```bash
curl --user admin:admin123 ftp://127.0.0.1:2121/
```

## Backend

O backend tem entrada em `cmd/ftm-backend` e implementacao em `internal/backend`. Ele e responsavel por gerenciar o servidor FTP.

Ele ja possui:

- login via `POST /api/login`;
- JWT assinado com HMAC-SHA256;
- permissao `admin` para acoes administrativas;
- persistencia binaria local em `encoding/gob`;
- criacao automatica do admin inicial;
- endpoints para consultar status e controlar o FTP.

Configuracao padrao:

```text
configs/backend.toml
```

Conta inicial:

```text
admin / admin123
```

Troque essa senha no primeiro uso criando/sobrescrevendo a conta admin via API.

Build:

```bash
mkdir -p build
go build -o build/ftm-backend ./cmd/ftm-backend
```

Ou compile ambos:

```bash
make build
```

Formatar o codigo Go:

```bash
make fmt
```

Rodar:

```bash
./build/ftm-backend -config ./configs/backend.toml
```

Login:

```bash
curl -s -X POST http://127.0.0.1:8080/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'
```

Consultar status:

```bash
curl http://127.0.0.1:8080/api/status \
  -H "Authorization: Bearer $TOKEN"
```

Iniciar o FTP pelo backend:

```bash
curl -X POST http://127.0.0.1:8080/api/ftp/start \
  -H "Authorization: Bearer $TOKEN"
```

Parar o FTP:

```bash
curl -X POST http://127.0.0.1:8080/api/ftp/stop \
  -H "Authorization: Bearer $TOKEN"
```

Criar conta do painel:

```bash
curl -X POST http://127.0.0.1:8080/api/accounts \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"username":"viewer","password":"viewer123","role":"viewer"}'
```

## Observacoes

- Esta primeira base nao usa banco externo nem dependencia Go de terceiros.
- O arquivo binario do backend fica em `data_file`, configurado em `configs/backend.toml`.
- O servidor FTP continua independente: ele pode rodar sem backend e sem dashboard.
- O backend controla o FTP como processo separado usando o binario configurado em `ftp_binary`.
