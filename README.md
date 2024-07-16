# Файлы для итогового задания
**Проект "Простой планировщик задач на Go"**

**_1. Описание проекта:_** 
Go веб-сервер, который реализует функциональность простейшего планировщика задач. Аналог TODO-листа. Планировщик хранит задачи, каждая из них содержит дату дедлайна и заголовок с комментарием. Задачи могут повторяться по заданному правилу. Если отметить такую задачу как выполненную, она переносится на следующую дату в соответствии с правилом. Обычные задачи при выполнении будут просто удаляться. 

    -API содержит следующие операции:
        добавить задачу;
        получить список задач;
        удалить задачу;
        получить параметры задачи;
        изменить параметры задачи;
        отметить задачу как выполненную.

    -В директории `tests` находятся тесты для проверки API, которое должно быть реализовано в веб-сервере.

    - Директория `web` содержит файлы фронтенда.

**_2. Список выполненных заданий со звёздочкой:_**
    Выполнены все задания со звёздочкой

**_3. Инструкция по запуску кода локально:_**
    -перейти в директорию backend и скомпилировать исполняемый файл командой:
    `go build`

    -запустить скомпилированный исполняемый файл командой:
    `./backend.exe`

**_4. Инструкция по запуску тестов._**
    - запустить код локально, см п.3

    - перейти по адресу:
    http://localhost:7540/

    - ввести пароль соответсвующий паременной окружения TODO_PASSWORD из докерфайла 

    - открыть консоль и во вкладке приложение открыть пункт "Файлы cookie". Скопировать значение куки token

    - вставить полученное значение в файл settings.go для переменной Token

    - далее открыть новый терминал и запустить тесты командой:
    `go test ./tests`

**_5. Инструкция по сборке и запуску проекта через докер:_**
    - перейти в корень проекта и запустить команду, чтобы сбилдить исполняемый файл:
    `docker build -t go_final_project .`

    - запустить образ из докер контейнера:
    `docker run -d --name go_final_project_server -p 7540:7540 go_final_project`

    - перейти по адресу:
    http://localhost:7540/





