package dates

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

const DateTemplate = "20060102"

// Условие: задача повторяется каждый раз через заданное кол-во дней
func CalculatDailyTask(now time.Time, startDate time.Time, repeat string) (string, error) {
	days, err := strconv.Atoi(strings.TrimPrefix(repeat, "d "))

	if err != nil {
		log.Printf("Error converting string to number:%s \n", err)
		return "", err
	}

	if days <= 0 || days > 400 {
		err = errors.New("invalid number of days")
		return "", err
	}

	var nextDate time.Time
	if days == 1 && now.Format(DateTemplate) == startDate.Format(DateTemplate) {
		nextDate = now
		return nextDate.Format(DateTemplate), nil
	}

	nextDate = startDate
	// Рассматриваем вариант, когда дата начала(starDate) задачи находится в будущем относительно текущего времени(now)
	if now.Before(nextDate) || now == nextDate {
		nextDate = nextDate.AddDate(0, 0, days)
		return nextDate.Format(DateTemplate), nil
	}

	// Рассматриваем вариант, когда дата начала задачи(starDate) находится в прошлом относительно текущего времени(now)
	for now.After(nextDate) || now == nextDate {
		nextDate = nextDate.AddDate(0, 0, days)
	}

	return nextDate.Format(DateTemplate), nil
}

// Условие: перенос задачи на год вперед
func CalculatYearlyTask(now time.Time, startDate time.Time) (string, error) {
	nextDate := startDate.AddDate(1, 0, 0)
	for now.After(nextDate) {
		nextDate = nextDate.AddDate(1, 0, 0)
	}
	return nextDate.Format(DateTemplate), nil
}

// Условие: перенос задачи на один из указанных дней недели
func CalculatWeeklyTask(now time.Time, startDate time.Time, repeat string) (string, error) {
	// Удаляем префикс, чтобы осталась часть строки с числами
	daysStr := strings.TrimPrefix(repeat, "w ")

	// Пробуем разделить сроку по запятым, чтобы получить массив из строк
	days := strings.Split(daysStr, ",")

	// Создаем новый массив для целых чисел
	var daysWeekNum []int

	// В цикле перебираем ранее полученный строковый массив days
	for _, dayStr := range days {
		// Форматируем строки в целые числа и добавляем в новый числовой массив
		dayNum, err := strconv.Atoi(dayStr)
		if err != nil {
			log.Printf("Error converting string to number: %s", err)
			return "", err
		}
		if dayNum < 1 || dayNum > 7 {
			err = errors.New("invalid number of days of weeks")
			return "", err
		}
		daysWeekNum = append(daysWeekNum, dayNum)
	}

	nextDate := startDate                         // Если now находится в прошлом относительно startDate
	if now == startDate || now.After(startDate) { // Если now равно startDate или если now в будущем относительно starDate
		nextDate = now.AddDate(0, 0, 1)
	} else if startDate.Before(now) {
		nextDate = startDate.AddDate(0, 0, 1)
	}

	// Перебираем дни после найденного nextDate и сравниваем с числами из массива
	// Заводим счетчик, чтобы цикл не выполнялся бесконечно
	counter := 0
	for nextDate.After(now) {
		if counter > 14 {
			return "", errors.New("next date for task not found")
		}

		for _, dayWeek := range daysWeekNum {
			if dayWeek == int(nextDate.Weekday()) || (dayWeek == 7 && nextDate.Weekday() == 0) {
				return nextDate.Format(DateTemplate), nil
			}
		}
		nextDate = nextDate.AddDate(0, 0, 1)
		counter++
	}
	return "", nil
}

// Определение нужной функции для определения ежемесячных задач
func CalculatMonthlyTask(now time.Time, startDate time.Time, repeat string) (string, error) {
	daysMonthStr := strings.TrimPrefix(repeat, "m ")
	daysMonth := strings.Split(daysMonthStr, " ")

	// Длина полученного слайса = 1, говорит о том, что мы имеем дело только с днями месяца
	if len(daysMonth) == 1 {
		days := daysMonth[0]
		daysStr := strings.Split(days, ",")

		var daysNum []int
		for _, day := range daysStr {
			dayNum, err := strconv.Atoi(day)
			if err != nil {
				log.Printf("Error converting string to number: %s", err)
				return "", err
			}

			if dayNum > -3 && dayNum < 32 {
				daysNum = append(daysNum, dayNum)
			} else {
				return "", fmt.Errorf("number greater than 32 or less than -3")
			}
		}

		return CalculatDayOfMonthTask(now, startDate, daysNum)
	}

	// Пока мы не знаем, какая перед нами комбинация
	// день + месяцы
	// дни + месяцы
	numList1 := strings.Split(daysMonth[0], ",")
	numList2 := strings.Split(daysMonth[1], ",")

	var daysNum []int
	for _, day := range numList1 {
		dayNum, err := strconv.Atoi(day)
		if err != nil {
			log.Printf("Error converting string to number: %s", err)
			return "", err
		}
		daysNum = append(daysNum, dayNum)
	}

	// Преобразуем элементы второго массива в числа
	// Создаем новый числовой массив
	var monthsNum []int
	for _, month := range numList2 {
		monthNum, err := strconv.Atoi(month)
		if err != nil {
			log.Printf("Error converting string to number: %s", err)
			return "", err
		}

		if monthNum < 1 && monthNum > 12 {
			return "", fmt.Errorf("month cannot be greater than 13 or a negative number")

		}
		monthsNum = append(monthsNum, monthNum)
	}

	return CalculatMonthsTask(now, startDate, daysNum, monthsNum)
}

// Условие: перенос задачи на заданный день месяца
func CalculatDayOfMonthTask(now time.Time, startDate time.Time, daysNum []int) (string, error) {
	nextDate, _ := PreliminaryNextDate(now, startDate)

	// Считаем кол-во отрицательных чисел в массиве
	negativeNum := CountNegativeNumbers(daysNum)

	// Ищем мининимально число в массиве daysNum
	minNumDay := FindMinNum(daysNum, negativeNum)

	return NowBeforeNextDate(now, nextDate, negativeNum, minNumDay, daysNum)
}

// Условие: перенос задачи на определенное число указанных месяцев
func CalculatMonthsTask(now time.Time, startDate time.Time, daysNum []int, monthsNum []int) (string, error) {
	nextDate, _ := PreliminaryNextDate(now, startDate)

	// Ищем подходящий месяц
	isLoop := true
	counter := 0

searchingMonths:
	for isLoop {
		for _, month := range monthsNum {
			if month == int(nextDate.Month()) && counter < 1 {
				isLoop = false
				break searchingMonths
			} else if month == int(nextDate.Month()) {
				nextDate = time.Date(nextDate.Year(), nextDate.Month(), 1, 0, 0, 0, 0, nextDate.Location())
				break searchingMonths
			}
		}
		nextDate = nextDate.AddDate(0, 1, 0)
		counter++
	}

	// Считаем кол-во отрицательных чисел в массиве
	negativeNum := CountNegativeNumbers(daysNum)

	// Ищем мининимально число в массиве daysNum
	minNumDay := FindMinNum(daysNum, negativeNum)

	return NowBeforeNextDate(now, nextDate, negativeNum, minNumDay, daysNum)
}

func NextDate(now time.Time, date string, repeat string) (string, error) {
	// Парсинг стартового-исходного времени, когда задача была выполнена первый раз
	startDate, err := time.Parse(DateTemplate, date)
	if err != nil {
		log.Printf("The start time cannot be converted to a valid date: %s", err)
		return "", err
	}

	switch {
	case strings.HasPrefix(repeat, "d "):
		return CalculatDailyTask(now, startDate, repeat) // Функция расчета даты для ежедневных задач
	case repeat == "y":
		return CalculatYearlyTask(now, startDate) // Функция расчета даты для ежегодных дел
	case strings.HasPrefix(repeat, "w "):
		return CalculatWeeklyTask(now, startDate, repeat) // Функция расчета даты для задач на определенные дни недели
	case strings.HasPrefix(repeat, "m "):
		return CalculatMonthlyTask(now, startDate, repeat) // Функция расчета даты для задач на определенные дни месяца
	case repeat == "":
		return "", errors.New("Repeat is empty")
	default:
		return "", errors.New("Repetition rule is not supported")
	}
}
