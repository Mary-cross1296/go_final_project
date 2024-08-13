package dates

import (
	"fmt"
	"math"
	"slices"
	"time"
)

func CountNegativeNumbers(nums []int) int {
	counterNegativeNum := 0
	for _, num := range nums {
		if num < 0 {
			counterNegativeNum++
		}
	}
	return counterNegativeNum
}

func FindMinNum(nums []int, numNegative int) int {
	minNumDay := math.MaxInt64

	if numNegative <= 1 {
		for _, minDay := range nums {
			if minDay > 0 && minDay < minNumDay {
				minNumDay = minDay
			}
		}
	} else if numNegative > 1 {
		minNumDay = 0
		for _, minDay := range nums {
			if minDay < 0 && minDay < minNumDay {
				minNumDay = minDay
			}
		}
	}
	return minNumDay
}

// Функция поиска предварительной nextDate
func PreliminaryNextDate(now time.Time, startDate time.Time) (time.Time, error) {
	nextDate := startDate
	if now == startDate || now.After(startDate) { // Now в будущем относительно startDate
		nextDate = now.AddDate(0, 0, 1)
		return nextDate, nil
	} else if now.Before(startDate) { // Now в прошлом относительно startDate
		nextDate = startDate.AddDate(0, 0, 1)
		return nextDate, nil
	}
	return time.Time{}, fmt.Errorf("TentativeNextDate() could not predetermine the next date")
}

func NowBeforeNextDate(now time.Time, nextDate time.Time, negativeNum int, minNumDay int, daysNum []int) (string, error) {
	// Расчет nextDate в функции PreliminaryNextDate, позволяет всегда выполняться условию now.Before(nextDate)
	for now.Before(nextDate) {
		for _, day := range daysNum {
			// Проверка даты для положительного числа из массива
			if day > 0 && negativeNum == 0 && day == nextDate.Day() {
				return nextDate.Format(DateTemplate), nil
			}

			if day > 0 && negativeNum == 1 && day == nextDate.Day() {
				negativeNumMin := slices.Min(daysNum)
				allegedNextDate := CalculatAllegedNextDate(nextDate, negativeNumMin)
				if nextDate.Day() <= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() {
					nextDate = time.Date(nextDate.Year(), nextDate.Month(), minNumDay, 0, 0, 0, 0, nextDate.Location())
					return nextDate.Format(DateTemplate), nil
				} else if nextDate.Day() >= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() {
					return allegedNextDate.Format(DateTemplate), nil
				}
				return nextDate.Format(DateTemplate), nil
			}

			if len(daysNum) == 1 && negativeNum == 1 {
				allegedNextDate := CalculatAllegedNextDate(nextDate, day)
				if nextDate.Day() <= allegedNextDate.Day() {
					return allegedNextDate.Format(DateTemplate), nil
				}
			}

			// Если день из массива отрицательное число при этом в массиве одно отрицательное число,
			// то вычисляем следующую дату задачи
			if day < 0 && negativeNum == 1 {
				allegedNextDate := CalculatAllegedNextDate(nextDate, day)
				if nextDate.Day() <= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() {
					nextDate = time.Date(nextDate.Year(), nextDate.Month(), minNumDay, 0, 0, 0, 0, nextDate.Location())
					return nextDate.Format(DateTemplate), nil
				} else if nextDate.Day() >= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() {
					return allegedNextDate.Format(DateTemplate), nil
				}
			}

			// Если день из массива отрицательное число (при этом в массиве два отрицательных числа),
			// то вычисляем следующую дату задачи следующим образом
			if day < 0 && negativeNum == 2 {
				day1 := daysNum[0]
				day2 := daysNum[1]

				allegedNextDate1 := CalculatAllegedNextDate(nextDate, day1)
				allegedNextDate2 := CalculatAllegedNextDate(nextDate, day2)

				// Вычисляем дату, которая происходит раньше
				if allegedNextDate1.Day() >= nextDate.Day() &&
					allegedNextDate2.Day() > nextDate.Day() {
					return allegedNextDate2.Format(DateTemplate), nil
				} else {
					return allegedNextDate1.Format(DateTemplate), nil
				}
			}
		}
		nextDate = nextDate.AddDate(0, 0, 1)
	}
	return "", nil
}

func CalculatAllegedNextDate(nextDate time.Time, day int) time.Time {
	// Устанавливаем предполагаемую следующую дату задания на 1 число текущего месяца
	allegedNextDate := time.Date(nextDate.Year(), nextDate.Month(), 1, 0, 0, 0, 0, nextDate.Location())
	// Переносим на первое число следующего месяца
	allegedNextDate = allegedNextDate.AddDate(0, 1, 0)
	// Из полученной даты вычетаем указанное кол-во дней
	// Получаем предполагаемую дату следующей задачи
	allegedNextDate = allegedNextDate.AddDate(0, 0, day)
	return allegedNextDate
}
