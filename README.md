# go-musthave-metrics-tpl

Шаблон репозитория для трека «Сервер сбора метрик и алертинга».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Аудит запросов

Аудит включается отдельно для каждого приёмника:

- `--audit-file` / `AUDIT_FILE` — путь к файлу JSON Lines; события добавляются в конец файла.
- `--audit-url` / `AUDIT_URL` — полный HTTP(S) URL; события отправляются методом POST с `Content-Type: application/json`.

Переменные окружения имеют приоритет над флагами. Без обоих параметров аудит отключён; можно включить один или оба приёмника.

```sh
go run ./cmd/server --audit-file ./audit.jsonl --audit-url http://localhost:9090/audit
```

После успешного обновления через `/update/{metricType}/{metricName}/{rawValue}`, `/update/` или `/updates/` сервер формирует одно событие на запрос:

```json
{"ts":12345678,"metrics":["Alloc","Frees"],"ip_address":"192.168.0.42"}
```

`ts` — Unix-время в секундах; IP берётся из адреса соединения без порта. За прокси это адрес прокси: заголовки перенаправления не используются.

Издатель уведомляет всех наблюдателей синхронно. Запись в файл защищена от одновременных записей, HTTP-доставка ограничена тайм-аутом 5 секунд. Ошибки аудита записываются в лог и не меняют результат обновления метрик; сбой одного наблюдателя не мешает вызову остальных. Повторных отправок нет.

## Бенчмарки и профили памяти

Бенчмарки находятся в `internal/performance/performance_test.go`:

- `BenchmarkServiceUpdateBatch` — валидация и сохранение пакета метрик через сервис.
- `BenchmarkServerSnapshot` — получение отсортированного снимка серверного хранилища.
- `BenchmarkAgentSnapshot` — получение независимого снимка хранилища агента.
- `BenchmarkPipeline` — отправка агентом JSON/gzip через настоящий loopback HTTP, распаковка и обработка `/updates/` сервером, сохранение в памяти и gzip-ответ.

Каждый пакет содержит 128 метрик четырёх экземпляров приложения: 96 gauge и 32 counter. Имена и значения соответствуют статистике runtime и счётчикам запросов. Хранилище заполняется до замеров, HTTP-соединение прогревается. Аудит, PostgreSQL и файловое сохранение в этой нагрузке отключены. Агент и тестовый сервер работают в одном процессе; это измерение выбранного пути обработки, а не всей производственной конфигурации.

### Методика

Для каждого состояния собирался отдельный тестовый бинарник. Например, исходный замер:

```sh
mkdir -p profiles
go test -c -o /tmp/metrics-base.test ./internal/performance
GOMAXPROCS=2 /tmp/metrics-base.test -test.run='^$' -test.bench=. \
  -test.benchmem -test.benchtime=1000x -test.count=3 > profiles/base-bench.txt
GOMAXPROCS=2 /tmp/metrics-base.test -test.run='^$' -test.bench='^BenchmarkPipeline$' \
  -test.benchtime=1000x -test.memprofilerate=1 \
  -test.memprofile=profiles/base.pprof > profiles/base-profile.txt
```

После оптимизации повторить команды с `result` вместо `base`. Для воспроизведения исходного замера нужен исходный gzip-код; запуск обеих команд на оптимизированном коде не воспроизводит сравнение до/после.

Профили снимаются сразу после нагрузки. Фиксированные 1000 итераций позволяют сравнивать одинаковый объём работы, а не разное количество запросов за одинаковое время. Профиль содержит также подготовку, прогрев и калибровочный запуск Go-бенчмарка. `memprofilerate=1` учитывает каждое выделение памяти и заметно замедляет выполнение, поэтому скорость оценивается по отдельным запускам без полного профилирования. `-race` при замерах не используется.

### Анализ pprof

Использованы представления `top`, `list`, `peek` и `web`:

```sh
go tool pprof -top -alloc_space profiles/base.pprof
go tool pprof -top -alloc_objects profiles/base.pprof
go tool pprof -top -inuse_space profiles/base.pprof
go tool pprof -list='compressedBody|gzipResponseWriter.*WriteHeader' -alloc_space profiles/base.pprof
go tool pprof -peek='compress/flate.NewWriter' -alloc_space profiles/base.pprof
go tool pprof -web -alloc_space -output=profiles/base-web.svg profiles/base.pprof
```

Для `web` требуется Graphviz (`dot`). Граф сохранён в `profiles/base-web.svg`; текстовые отчёты — в `profiles/base-{top,objects,inuse,list,peek}.txt`. Отчёт `list` сохранён до редактирования исходников, чтобы номера строк и код соответствовали исходному профилю.

`top` и `peek` показали, что `compress/flate.NewWriter` вместе с инициализацией компрессора отвечает за 92,06% выделенных байтов. `list` связал эти выделения с созданием нового gzip-писателя в `sender.compressedBody` и серверном `gzipResponseWriter`. Основные буферы выделяются при первом `Write` или `Close`, поэтому в графе сервера значительная часть затрат приходится на `Close`, даже для пустого тела ответа. По количеству объектов основной источник — декодирование JSON, прежде всего строки и значения метрик.

Оптимизация: общий вспомогательный пакет `internal/gziputil` выдаёт gzip-писатели из `sync.Pool`. Агент и middleware возвращают писатель после завершения потока. Перед возвратом `Reset(io.Discard)` отсоединяет предыдущий буфер или HTTP-ответ; один писатель принадлежит только одному запросу. Формат JSON, уровень сжатия и HTTP-заголовки сохранены. Добавлены проверки независимости параллельных потоков и восстановления после ошибки записи.

### Результаты

Медианы трёх запусков, каждый по 1000 итераций; исходные измерения сохранены в `profiles/base-bench.txt` и `profiles/result-bench.txt`:

| Бенчмарк | ns/op до → после | B/op до → после | allocs/op до → после |
| --- | ---: | ---: | ---: |
| ServiceUpdateBatch | 4 698 → 2 844 | 0 → 0 | 0 → 0 |
| ServerSnapshot | 16 546 → 15 916 | 10 632 → 10 632 | 132 → 132 |
| AgentSnapshot | 14 641 → 13 363 | 15 464 → 15 464 | 133 → 133 |
| Pipeline | 337 165 → 293 481 | 1 752 150 → 119 027 | 577 → 532 |

Для Pipeline выделение байтов уменьшилось на **93,2%**, число аллокаций — на **7,8%**, время — примерно на **13%**. Короткие замеры времени шумные: изменение скорости неизменённых компонентов не следует приписывать оптимизации gzip.

В полных профилях `alloc_space` уменьшился с 1 716 874,25 до 114 211,23 КиБ (−93,35%), `alloc_objects` — с 440 241 до 395 488. Это накопленные выделения за запуск, а не размер одновременно занятой памяти и не RSS процесса.

Удерживаемая память (`inuse_space`) после нагрузки **выросла** со 118,99 до 1 703,65 КиБ: пул сохранил рабочие буферы компрессоров, около 1,55 МиБ дополнительно. Это явный компромисс ради меньшего объёма выделений и нагрузки на GC; уменьшение удерживаемой памяти этим замером не доказано. `sync.Pool` может удалять неиспользуемые объекты при сборке мусора и не гарантирует фиксированный размер. Отдельные отчёты сохранены в `profiles/base-inuse.txt`, `profiles/result-inuse.txt` и `profiles/diff-inuse.txt`.

Команда сравнения из задания (встроенный `go tool pprof` эквивалентен отдельному `pprof`):

```sh
go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
```

Фактический вывод, также сохранённый в `profiles/diff.txt`. В этих профилях тип по умолчанию — `alloc_space`; отрицательные значения означают уменьшение накопленных выделений:

```text
File: metrics-result.test
Type: alloc_space
Time: 2026-10-05 17:38:57 MSK
Showing nodes accounting for -1581552.28kB, 92.12% of 1716874.25kB total
Dropped 182 nodes (cum <= 8584.37kB)
      flat  flat%   sum%        cum   cum%
-1297944kB 75.60% 75.60% -1578113.73kB 91.92%  compress/flate.NewWriter (inline)
-272408.11kB 15.87% 91.47% -272408.11kB 15.87%  compress/flate.(*compressor).initDeflate (inline)
  -11200kB  0.65% 92.12%   -11200kB  0.65%  net/http.init.func16
   -0.22kB 1.3e-05% 92.12% -11696.93kB  0.68%  net/http.(*persistConn).writeLoop
    0.05kB 2.7e-06% 92.12% -11696.71kB  0.68%  net/http.(*Request).write
         0     0% 92.12% -280169.73kB 16.32%  compress/flate.(*compressor).init
         0     0% 92.12% -795412.98kB 46.33%  compress/gzip.(*Writer).Close
         0     0% 92.12% -1578113.73kB 91.92%  compress/gzip.(*Writer).Write
         0     0% 92.12% -793047.72kB 46.19%  encoding/json.(*Encoder).Encode
         0     0% 92.12% -789009.96kB 45.96%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0% 92.12% -800300.05kB 46.61%  github.com/tomkqwe/metrics/internal/agent/sender.(*HTTPSender).Send
         0     0% 92.12% -799953.22kB 46.59%  github.com/tomkqwe/metrics/internal/agent/sender.compressedBody
         0     0% 92.12% -788662.98kB 45.94%  github.com/tomkqwe/metrics/internal/middleware.(*gzipResponseWriter).Close
         0     0% 92.12% -788806.42kB 45.94%  github.com/tomkqwe/metrics/internal/middleware.WithGzip.func1
         0     0% 92.12% -788662.98kB 45.94%  github.com/tomkqwe/metrics/internal/middleware.WithGzip.func1.2
         0     0% 92.12% -800300.02kB 46.61%  github.com/tomkqwe/metrics/internal/performance_test.BenchmarkPipeline
         0     0% 92.12% -790650.80kB 46.05%  net/http.(*conn).serve
         0     0% 92.12% -11350.30kB  0.66%  net/http.(*transferWriter).doBodyCopy
         0     0% 92.12% -11350.30kB  0.66%  net/http.(*transferWriter).writeBody
         0     0% 92.12% -788806.42kB 45.94%  net/http.HandlerFunc.ServeHTTP
         0     0% 92.12% -11350.61kB  0.66%  net/http.getCopyBuf (inline)
         0     0% 92.12% -789009.96kB 45.96%  net/http.serverHandler.ServeHTTP
         0     0% 92.12% -13326.61kB  0.78%  sync.(*Pool).Get
         0     0% 92.12% -799497.68kB 46.57%  testing.(*B).launch
         0     0% 92.12% -800300.27kB 46.61%  testing.(*B).runN
```

Проверки после изменения: `go test -race ./...`, `go vet ./...`.

## Документация Go и примеры HTTP API

Публичные типы, интерфейсы, функции и методы документированы комментариями godoc.
Просмотр документации из корня проекта:

```sh
go doc ./internal/handler
go doc ./internal/handler.MetricsHandler.UpdateMetricsJSON
go doc ./internal/service.Service
```

В `internal/handler/example_test.go` находятся исполняемые примеры обновления метрик
через URL и JSON, пакетного обновления, чтения значений, HTML-списка и `/ping`,
а также ответов 400 и 404. Примеры используют настоящие хендлеры и хранилище в памяти;
для `/ping` проверку соединения заменяет тестовая реализация `DatabasePinger`.
Внешний HTTP-сервер и PostgreSQL не требуются. Секции `Output` проверяются автоматически:

```sh
go test ./internal/handler -run Example -v
```
