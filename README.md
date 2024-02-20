# Drawnstories-CBZ

Этот код предназначен для создания CBZ книг из изображений скачанных с
сайта[drawnstories.ru](https://drawnstories.ru/). Код реализует утилиту командной строки.

## Использование

Для использования программы на [странице релизов](https://github.com/sattellite/drawnstories-cbz/releases)
скачайте бинарный файл для вашей операционной системы.

В терминале запустите его с указанием адреса комикса и какие выпуски надо скачать:

```bash
drawnstories-cbz https://drawnstories.ru/comics/Oni-press/rick-and-morty 001 002 003
```

Если надо скачать все выпуски со страницы, то номера комиксов указывать не обязательно:

```bash
drawnstories-cbz https://drawnstories.ru/comics/Oni-press/rick-and-morty
```
