# Network Infrastructure Monitoring Agent

Go-агент мониторинга сетевой инфраструктуры с экспортом метрик в формате Prometheus.

## Архитектура

```
agent/
├── cmd/agent/main.go           # Точка входа
├── internal/
│   ├── config/     config.go   # Загрузка YAML-конфига
│   ├── ping/       ping.go     # ICMP-проверка узлов
│   ├── scheduler/  scheduler.go# Worker pool + ticker
│   ├── metrics/    metrics.go  # Prometheus gauges
│   ├── exporter/   exporter.go # HTTP /metrics endpoint
│   └── logger/     logger.go   # Структурированное логирование
├── configs/config.yaml         # Конфигурация по умолчанию
├── deploy/
│   ├── prometheus/             # prometheus.yml + rules.yml
│   ├── alertmanager/           # alertmanager.yml (Telegram)
│   └── grafana/                # Datasource + Dashboard provisioning
├── Dockerfile
└── docker-compose.yml
```

## Быстрый старт

### 1. Запуск полного стека (рекомендуется)

```bash
# Задайте переменные Telegram для алертов:
export TELEGRAM_BOT_TOKEN=<your_token>
export TELEGRAM_CHAT_ID=<your_chat_id>

docker compose up -d
```

| Сервис       | URL                        |
|--------------|----------------------------|
| Агент        | http://localhost:9100/metrics |
| Prometheus   | http://localhost:9090      |
| Grafana      | http://localhost:3000      |
| Alertmanager | http://localhost:9093      |

Grafana: логин `admin` / пароль `admin`.

### 2. Запуск агента локально (нужен Go 1.22+)

```bash
# Требуется NET_RAW для ICMP (sudo или CAP_NET_RAW)
sudo go run ./cmd/agent -config configs/config.yaml
```

### 3. Сборка Docker-образа

```bash
docker build -t go-agent .
docker run --cap-add NET_RAW -p 9100:9100 go-agent
```

## Конфигурация

`configs/config.yaml`:

```yaml
targets:
  - 8.8.8.8
  - 1.1.1.1
  - 8.8.4.4

interval: 5s    # Интервал между проверками
timeout:  2s    # Таймаут одного ping
port:     9100  # Порт экспортёра
workers:  10    # Размер worker pool
```

## Метрики

```
# HELP network_up Whether the target host is reachable (1 = up, 0 = down).
network_up{target="8.8.8.8"} 1

# HELP network_latency_ms Average round-trip time in milliseconds.
network_latency_ms{target="8.8.8.8"} 24

# HELP network_packet_loss Percentage of packets lost (0-100).
network_packet_loss{target="8.8.8.8"} 0
```

## Alert Rules

| Alert         | Условие                          | Severity |
|---------------|----------------------------------|----------|
| HostDown      | `network_up == 0` (1 мин)        | critical |
| HighLatency   | `network_latency_ms > 100` (2 мин)| warning  |
| PacketLoss    | `network_packet_loss > 20` (2 мин)| warning  |

Уведомления отправляются в Telegram через Alertmanager.

## Concurrency-модель

```
Scheduler.runOnce()
    │
    ├─ taskCh (buffered channel, все таргеты)
    │
    ├─ Worker goroutine 1 ── Pinger.Ping("8.8.8.8")
    ├─ Worker goroutine 2 ── Pinger.Ping("1.1.1.1")
    └─ Worker goroutine N ── Pinger.Ping("...")
               │
               └─ metrics.Update() ── gauge.set() [sync.RWMutex]
```

- `goroutines` — параллельный обход всех узлов
- `channel` — очередь задач, синхронизация без мьютексов на уровне планировщика
- `sync.WaitGroup` — ожидание завершения раунда
- `worker pool` — ограничение параллелизма (`workers` в конфиге)

## Тесты

```bash
go test ./...
```

## Технологический стек

| Компонент      | Технология            |
|----------------|-----------------------|
| Агент          | Go 1.22 (stdlib only) |
| Мониторинг     | Prometheus            |
| Визуализация   | Grafana               |
| Алертинг       | Alertmanager + Telegram|
| Контейнеры     | Docker / Compose      |
