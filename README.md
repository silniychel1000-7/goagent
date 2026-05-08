# Network Infrastructure Monitoring System

Go-агент мониторинга сетевой инфраструктуры с эмуляцией сети через Containerlab.

## Архитектура

```
agent/
├── cmd/agent/main.go               # Точка входа
├── internal/
│   ├── config/      config.go      # Загрузка YAML-конфига
│   ├── ping/        ping.go        # ICMP + TCP fallback
│   ├── scheduler/   scheduler.go   # Worker pool + ticker
│   ├── metrics/     metrics.go     # Prometheus gauges
│   ├── exporter/    exporter.go    # HTTP /metrics endpoint
│   ├── logger/      logger.go      # Структурированное логирование
│   ├── collector/
│   │   ├── iface.go               # /proc/net/dev metrics
│   │   └── tcp.go                 # TCP port probes
│   └── jitter/      jitter.go     # RTT jitter (RFC 3550)
├── configs/config.yaml
└── deploy/
    ├── containerlab/
    │   ├── topology.clab.yml      # R1, R2, R3, agent
    │   └── config/{r1,r2,r3}/    # FRRouting OSPF configs
    ├── prometheus/                # prometheus.yml + rules.yml
    ├── alertmanager/              # alertmanager.yml (Telegram)
    └── grafana/                   # Datasource + Dashboard
scripts/
└── fault_demo.sh                  # Demo для защиты диплома
```

## Метрики

| Метрика | Описание |
|---------|----------|
| `network_up{target}` | Хост доступен (1/0) |
| `network_latency_ms{target}` | Средний RTT, мс |
| `network_packet_loss{target}` | Потери пакетов, % |
| `network_jitter_ms{target}` | Jitter (MAD), мс |
| `iface_rx_bytes_total{iface}` | Принято байт |
| `iface_tx_bytes_total{iface}` | Передано байт |
| `iface_rx_errors_total{iface}` | Ошибки RX |
| `iface_rx_drops_total{iface}` | Дропы RX |
| `tcp_up{target,port,service}` | TCP порт доступен (1/0) |
| `tcp_rtt_ms{target,port,service}` | TCP RTT, мс |

## Alert Rules

| Alert | Условие | Severity |
|-------|---------|----------|
| HostDown | `network_up == 0` (1 мин) | critical |
| HighLatency | `latency_ms > 100` (2 мин) | warning |
| PacketLoss | `packet_loss > 10%` (2 мин) | warning |
| HighJitter | `jitter_ms > 20` (2 мин) | warning |
| InterfaceErrors | `rate(rx_errors[5m]) > 1` | warning |
| TCPPortDown | `tcp_up == 0` (1 мин) | warning |

## Быстрый старт

### 1. Запуск с Containerlab (полный стек)

```bash
# Установка Containerlab (если не установлен)
bash -c "$(curl -sL https://get.containerlab.dev)"

# Запуск топологии
cd deploy/containerlab
sudo containerlab deploy -t topology.clab.yml

# Запуск мониторинга
export TELEGRAM_BOT_TOKEN=<token>
export TELEGRAM_CHAT_ID=<chat_id>
docker compose up -d
```

### 2. Запуск только агента локально

```bash
sudo go run ./cmd/agent -config configs/config.yaml
```

### 3. Сборка Docker образа

```bash
docker build -t go-agent .
docker run --cap-add NET_RAW -p 9100:9100 go-agent
```

## Сервисы

| Сервис | URL | Логин |
|--------|-----|-------|
| Agent metrics | http://localhost:9100/metrics | — |
| Prometheus | http://localhost:9090 | — |
| Grafana | http://localhost:3000 | admin / admin |
| Alertmanager | http://localhost:9093 | — |

## Демо для защиты диплома

```bash
# Запуск сценария fault injection
./scripts/fault_demo.sh

# Или вручную:
# 1. Посмотреть текущее состояние
curl http://localhost:9100/metrics | grep network_up

# 2. Сломать линк R1-R2
docker exec clab-netmon-r1 ip link set eth1 down

# 3. Наблюдать в Grafana: packet loss spike, latency rerouting
# 4. Проверить алерт в Telegram через ~1 мин

# 5. Восстановить
docker exec clab-netmon-r1 ip link set eth1 up
```

## Модель конкурентности

```
Scheduler.runOnce()
    │
    ├─ taskCh (buffered channel, все таргеты)
    │
    ├─ Worker goroutine 1 ── Pinger.Ping() ── Jitter.Add() ── metrics.Update()
    ├─ Worker goroutine 2 ── Pinger.Ping() ── Jitter.Add() ── metrics.Update()
    └─ Worker goroutine N ── ...
    
Background goroutine:
    └─ ticker → InterfaceCollector.Collect() + TCPProbe.Probe()
```

## Тесты

```bash
go test ./...
```

## Технологический стек

| Компонент | Технология |
|-----------|------------|
| Агент | Go 1.22 (stdlib only) |
| Сетевая эмуляция | Containerlab + FRRouting |
| Routing protocol | OSPF (FRR) |
| Мониторинг | Prometheus |
| Визуализация | Grafana |
| Алертинг | Alertmanager + Telegram |
| Контейнеры | Docker / Compose |
