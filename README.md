# 1C EDI BLRWBL Load Tester (Сервис нагрузочного тестирования)

Веб-сервис на Go для нагрузочного тестирования HTTP-сервисов 1C при интеграции EDI (прием электронных накладных BLRWBL).

Сервис отправляет документы XML `<BLRWBL>` методом `POST` на эндпоинт 1С `/api/v1/edocuments` и анализирует ответы 1C:
- **`201 Created`** (успех)
- **`400 Bad Request`** (ошибка структуры)
- Тело ответа 1C: JSON статусное сообщение (`Operation: 5`, `ProcessingStatus`, `StatusDescription`).

---

## Возможности

1. **Генерация уникальных XML на основе шаблона:**
   - Шаблон по умолчанию: `blrwbl 517_001-4810413900004-260616000002408.xml`.
   - Автоматический инкремент уникальных полей:
     - `MessageID`: например, базовый `00000000007712` + суффикс `001` → `00000000007712001`, следующий `00000000007712002` и т.д.
     - `DocumentID`: `00000000007712001`, `00000000007712002`...
     - `DeliveryNoteID`: `001-<GLN>-260616000002408001`, `001-<GLN>-260616000002408002`...
     - `GLN`: при указании кастомного GLN обновляются `MsgSenderID`, `Shipper -> GLN`, `FreightPayer -> GLN` и подстрока GLN в номере накладной `DeliveryNoteID`.
   - Опция автоматического обновления даты и времени формирования документа (`MsgDateTime`, `CreationDateTime`, `DeliveryNoteDate`).

2. **Гибкие параметры нагрузки:**
   - Настраиваемый целевой эндпоинт (по умолчанию `1C: /api/v1/edocuments` или встроенный Mock).
   - Задаваемое количество документов (`Count`).
   - Настраиваемая параллельность (количество горутин/потоков `Concurrency`).
   - Ограничение скорости (задержка между запросами в миллисекундах или максимальный RPS).
   - Поддержка авторизации в 1C: **Basic Auth** (логин/пароль) и **Bearer Token**.
   - Таймауты запросов и пулы соединений (HTTP Keep-Alive).

3. **Интерактивный веб-интерфейс (Single-Page Application):**
   - Управление запуском и принудительной остановкой теста (`Start` / `Stop`).
   - Модальное окно **Предпросмотра XML** (`Preview`) перед отправкой.
   - Мониторинг в реальном времени через **Server-Sent Events (SSE)**.
   - Прогресс-бар, счетчики `201 Created`, `400 Bad Request`, `Ошибки сети / 5xx`.
   - Расчет метрик скорости (текущий `RPS`) и задержек (`Min`, `Avg`, `Max`, `P95`).
   - Интерактивный журнал последних запросов с просмотром отправленного XML и полученного JSON от 1C.

4. **Встроенный Mock-сервер 1C:**
   - Эндпоинт `/mock/1c/api/v1/edocuments`.
   - Позволяет полноценно тестировать сервис без развернутой базы 1С.
   - Имитирует задержку обработки в 1С (`latency`) и процент ошибок разбора структуры (`errorRate`).

---

## Быстрый старт

### 1. Запуск сервиса

```bash
# Запуск скомпилированного бинарного файла:
./loadtester -port 8080

# Или запуск через go run:
go run main.go -port 8080
```

Флаги запуска:
- `-port <число>`: порт веб-сервиса (по умолчанию `8080`).
- `-template <путь>`: путь к эталонному XML-файлу (по умолчанию `blrwbl 517_001-4810413900004-260616000002408.xml`).

После запуска откройте в браузере: **`http://localhost:8080/`**

---

## Использование через REST API

Сервис также поддерживает управление через HTTP API:

### 1. Запуск нагрузочного теста (`POST /api/test/start`)
```bash
curl -X POST http://localhost:8080/api/test/start \
  -H "Content-Type: application/json" \
  -d '{
    "endpointUrl": "http://1c-server/EDI_exchange/api/v1/edocuments",
    "count": 100,
    "concurrency": 10,
    "gln": "4810413900004",
    "suffix": "001",
    "baseMessageID": "00000000007712",
    "baseDocumentID": "00000000007712",
    "baseDeliveryNoteID": "001-4810413900004-260616000002408",
    "delayMs": 0,
    "timeoutSec": 30,
    "updateTimestamps": true,
    "authType": "basic",
    "username": "edi_user",
    "password": "secret_password"
  }'
```

### 2. Получение текущего статуса и метрик (`GET /api/test/status`)
```bash
curl http://localhost:8080/api/test/status
```

### 3. Поток метрик в реальном времени (`GET /api/test/events` - SSE)
```bash
curl -N http://localhost:8080/api/test/events
```

### 4. Остановка теста (`POST /api/test/stop`)
```bash
curl -X POST http://localhost:8080/api/test/stop
```

### 5. Предпросмотр сформированных XML (`POST /api/test/preview`)
```bash
curl -X POST http://localhost:8080/api/test/preview \
  -H "Content-Type: application/json" \
  -d '{
    "gln": "4810413900004",
    "suffix": "001",
    "count": 2
  }'
```

---

## Запуск тестов

```bash
go test -v ./...
```
