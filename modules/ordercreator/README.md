# Ordercreator service

Оркестратор создания заказа на Temporal. Модуль также сохраняет echo-пример шаблона
(HTTP, gRPC, outbox, message-handler).

## Создание заказа

`ordercreator-service` принимает `POST /api/public/v1/orders` и запускает workflow
`CreateOrderWorkflow` с ID `order-<orderID>` в очереди `ordercreator`.
`ordercreator-worker` (команда `worker`) выполняет workflow: активности
последовательно вызывают gRPC internal API сервисов order, payment и delivery.
Код workflow и активностей находится в `internal/ordercreator/infra/orderflow`.

```mermaid
sequenceDiagram
  actor U as User
  participant GW as Gateway
  participant OCS as ordercreator-service<br/>(HTTP + Temporal client)
  participant TS as Temporal (temporal:7233)
  participant OCW as ordercreator-worker<br/>(workflow + activities)
  participant O as order-service :8081
  participant P as payment-service :8081
  participant D as delivery-service :8081

  U->>GW: POST /ordercreator/api/public/v1/orders {amount, address}
  GW->>OCS: POST /api/public/v1/orders
  OCS->>TS: ExecuteWorkflow(CreateOrderWorkflow, ID=order-<uuid>)
  OCS-->>U: 202 {orderID, workflowID}
  TS->>OCW: Activity task CreateOrder
  OCW->>O: gRPC OrderService.CreateOrder
  O-->>OCW: OK
  TS->>OCW: Activity task ChargePayment
  OCW->>P: gRPC PaymentService.Charge
  P-->>OCW: OK
  TS->>OCW: Activity task ScheduleDelivery
  OCW->>D: gRPC DeliveryService.Schedule
  D-->>OCW: OK
```

Каждая активность ограничена 10 секундами и повторяется до 5 раз с начальным
интервалом 1 секунда. Ответ `InvalidArgument` останавливает повторы. Сервисы
сохраняют запись идемпотентно по `order_id`, поэтому повтор активности не создаёт
дубликат.

```sh
curl -i -X POST localhost/ordercreator/api/public/v1/orders \
  -H 'Content-Type: application/json' -d '{"amount":100,"address":"Kazan"}'
```

Ход выполнения виден в Temporal UI: `http://localhost:8080`.

## Сценарий

```text
HTTP / gRPC → app → unit of work → echo + outbox (одна MySQL-транзакция)
                                             ↓
                                      relay → RabbitMQ
                                             ↓
                              message-handler → inbox → лог
```

HTTP: `POST /api/public/v1/echo` с JSON `{"body":"hello"}`. Ответ:
`{"operationID":"<uuid>"}`. Пустая строка допустима; поле `body` обязательно.

gRPC: `ordercreatorInternalAPI.OrdercreatorService/Echo`, запрос `{"Body":"hello"}`,
ответ с `OperationID`. Сервер поддерживает reflection:

```sh
grpcurl -plaintext -d '{"Body":"hello"}' localhost:8081 \
  ordercreatorInternalAPI.OrdercreatorService/Echo
```

Событие `ordercreatorEvents.Echo` содержит ID echo и исходную строку. SDK добавляет
envelope с event ID, producer, type, временем и correlation ID. Общий exchange
`domain-event` находится в `lib/event`; routing key модуля — `ordercreator.Echo`.

## Структура

- `cmd`: конфигурация, команды запуска и файлы сборки зависимостей по подсистемам: MySQL, миграции, AMQP, события и серверы.
- `internal/ordercreator/app`: транзакционная граница через SDK `UnitOfWorkWithRepositoryProvider`.
- `internal/ordercreator/domain`: echo-модель, бизнес-операция, интерфейс репозитория и payload события; используется нативный пакет `uuid` Go 1.27.
- `internal/ordercreator/infra/mysql`: repository provider; `repository` содержит реализацию echo-репозитория, `migrations` — миграции модуля.
- `internal/ordercreator/infra/handlers`: HTTP, gRPC и события.
- `internal/integration-tests`: инфраструктура, контейнеры и интеграционные проверки.
- `api`: protobuf/OpenAPI-контракты; generated Go-код исключён из Git.
- `config/gateway`: маршруты и объявления интерцепторов модуля для общего gateway.

Repository provider хранит транзакционный клиент и выдаёт репозиторий с контекстом
операции. `app` использует `ExecuteWithRepositoryProvider`; SQL-клиент в него не
передаётся. Outbox и репозиторий используют один context и один экземпляр UOW.

UUID в таблице `echo` хранится как `BINARY(16)`. В SQL-параметры передаётся
`uuid.UUID`: MySQL-клиент SDK преобразует его в 16 байт. В HTTP, gRPC и событиях
ID остаётся строкой.

## Пример интерцептора gateway

`internal/ordercreator/infra/handlers/gateway` реализует gRPC API интерцептора через
`lib/gateway/interceptor` и регистрируется на сервере
модуля (`8081`) вместе с internal API. `config/gateway/interceptor.yaml` объявляет
интерцептор, а `routes.yaml` подключает его к маршруту `/ordercreator/api/...`.
`local-up` автоматически собирает оба файла в конфигурацию gateway.

Пример показывает три решения:

- `requestHeaders`: устанавливает `X-Demo-Request-ID` из `context.requestId`, заменяя значение клиента.
- `responseHeaders`: добавляет `X-Demo-Interceptor: ordercreator` в ответ backend.
- `requestHeaders` с `X-Demo-Deny: true`: возвращает JSON с HTTP `403` через `immediateResponse`; backend и response-фаза не вызываются.

Это демонстрация API. Проверку заголовка `X-Demo-Deny` следует заменить политикой
приложения, если нужна авторизация или другая бизнес-проверка.

После `mise run local-up`:

```sh
curl -i -X POST localhost/ordercreator/api/public/v1/echo \
  -H 'Content-Type: application/json' -d '{"body":"through gateway"}'
curl -i -X POST localhost/ordercreator/api/public/v1/echo \
  -H 'Content-Type: application/json' -H 'X-Demo-Deny: true' -d '{"body":"denied"}'
```

Первый ответ содержит `X-Demo-Interceptor: ordercreator`, второй — `403`.
Для отключения примера удалите `interceptors` у маршрута. При обращении к HTTP
порту модуля напрямую интерцептор не вызывается.

## Запуск

Из корня репозитория:

```sh
mise run local-up
```

Корневой mise автоматически подключает модульный `compose.yaml`, собирает бинарник
и поднимает сервис и consumer в контейнерах.

Конфиги из `config/gateway/` автоматически собираются вместе с конфигами
остальных модулей при `local-up`. Общие настройки gateway находятся в корневом
`config/gateway/`. Маршрут `/ordercreator/api/...` проксируется на
`ordercreator-service:8082`, удаляя префикс `/ordercreator`:

```sh
curl -X POST localhost/ordercreator/api/public/v1/echo \
  -H 'Content-Type: application/json' -d '{"body":"hello"}'
```

После изменения конфигов повторите `mise run local-up` для перезапуска gateway.

Бинарник поддерживает `service` (по умолчанию), `message-handler` и `worker`.
`service` запускает HTTP, gRPC и relay. `message-handler` запускает consumer
и HTTP `/healthz`. `worker` запускает Temporal worker и HTTP `/healthz`. Health endpoint отвечает 204 и проверяет liveness.

## Конфигурация

| Переменная | Значение по умолчанию |
| --- | --- |
| `ORDERCREATOR_APP_DEBUG` | `false` |
| `ORDERCREATOR_APP_GRACE_TIMEOUT` | `30s` |
| `ORDERCREATOR_HTTP_ADDRESS` | `:8082` |
| `ORDERCREATOR_GRPC_ADDRESS` | `:8081` |
| `ORDERCREATOR_MYSQL_ADDRESS` | `localhost:3306` |
| `ORDERCREATOR_MYSQL_DATABASE` | `ordercreator` |
| `ORDERCREATOR_MYSQL_USER` | `ordercreator` |
| `ORDERCREATOR_MYSQL_PASSWORD` | `ordercreator` |
| `ORDERCREATOR_MYSQL_MAX_CONNECTIONS` | `10` |
| `ORDERCREATOR_MYSQL_CONNECTION_MAX_LIFETIME` | `30m` |
| `ORDERCREATOR_MYSQL_CONNECTION_MAX_IDLE_TIME` | `5m` |
| `ORDERCREATOR_MYSQL_CONNECT_TIMEOUT` | `5s` |
| `ORDERCREATOR_MYSQL_READ_TIMEOUT` | `10s` |
| `ORDERCREATOR_MYSQL_WRITE_TIMEOUT` | `10s` |
| `ORDERCREATOR_AMQP_URL` | `amqp://local:local@localhost:5672/` |
| `ORDERCREATOR_AMQP_QUEUE` | `ordercreator.echo` |
| `ORDERCREATOR_AMQP_CONNECT_TIMEOUT` | `5s` |
| `ORDERCREATOR_AMQP_PREFETCH_COUNT` | `10` |
| `ORDERCREATOR_RELAY_INTERVAL` | `1s` |
| `ORDERCREATOR_RELAY_BATCH_SIZE` | `100` |
| `ORDERCREATOR_RELAY_PUBLISH_TIMEOUT` | `10s` |
| `ORDERCREATOR_RELAY_LOCK_TIMEOUT` | `1s` |
| `ORDERCREATOR_TEMPORAL_ADDRESS` | `localhost:7233` |
| `ORDERCREATOR_TEMPORAL_NAMESPACE` | `default` |
| `ORDERCREATOR_TEMPORAL_TASK_QUEUE` | `ordercreator` |
| `ORDERCREATOR_SERVICES_ORDER_ADDRESS` | `localhost:8081` |
| `ORDERCREATOR_SERVICES_PAYMENT_ADDRESS` | `localhost:8081` |
| `ORDERCREATOR_SERVICES_DELIVERY_ADDRESS` | `localhost:8081` |

DSN формируется рядом с connector через `mysqldriver.Config.FormatDSN()`;
`parseTime` и `utf8mb4` включаются там же. `--debug` также включает подробные логи.
Новые модули используют свой env-префикс.

## Миграции и сборка

```sh
mise run //modules/ordercreator:generate-migration "Create a new table"
mise run //modules/ordercreator:build
```

Генератор создаёт и регистрирует новую миграцию. Реализуйте `Up`: начальная
заглушка возвращает ошибку. Обе команды сервиса применяют все таргеты миграций:
модуль, outbox и inbox.

`build` генерирует контракты и локально собирает Linux-бинарник `dist/ordercreator`.
Dockerfile только копирует его в образ. Для нативного запуска используйте задачу
`run`, для выбора архитектуры Linux-сборки — `GOARCH`.

## Гарантии

Успех API означает commit echo и outbox. Ошибка внутри транзакции откатывает обе
записи. Повторный API-запрос создаёт новый echo.

Оба процесса объявляют durable-очередь. Relay использует publisher confirms и
повторяет неудавшиеся публикации. API сохраняет операции при потере RabbitMQ
после запуска; backlog отправляется после восстановления соединения. Ошибка
подключения при первоначальном запуске останавливает процесс.

Доставка at-least-once. Inbox подавляет повторную обработку зафиксированных
`(consumer, event_id)`. Логирование не атомарно с commit inbox: при сбое возможен
повторный лог. Ошибки consumer и невалидные события попадают в `<queue>.dead`
через общий exchange `domain-event.dead`. Автоматический retry consumer не включён.

HTTP и gRPC принудительно закрываются после grace timeout и возвращают ошибку,
для которой `errors.Is(err, context.DeadlineExceeded)` истинно.
Очистка outbox/inbox через SDK `Cleanup` в пример не входит.

## Проверки

```sh
mise run //modules/ordercreator:all
mise run //modules/ordercreator:integration-tests
```

Интеграционные тесты проверяют HTTP и gRPC API, запись события в outbox,
обработку через inbox, повторную доставку и некорректные HTTP-запросы.
Задача mise собирает Linux-бинарник и Docker-образ. Testcontainers запускает
сервис и consumer из этого образа вместе с MySQL и RabbitMQ в отдельной сети.
Тестовый MySQL-пароль содержит специальные символы для проверки DSN.
