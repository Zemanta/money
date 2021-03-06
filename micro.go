package money

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	precisionExp                   = int64(6)
	precision                      = Micro(1000000)
	MaxMicro                       = math.MaxInt64
	MinMicro                       = math.MinInt64
	Zero                           = Micro(0)
	MicroDollar              Micro = 1
	Cent                           = 10000 * MicroDollar
	Dollar                         = 100 * Cent
	RoundingNone                   = 0
	RoundingHalfAwayFromZero       = 1
)

var ErrInvalidInput = errors.New("money: cannot convert string to money.Micro")
var ErrOverflow = errors.New("money: overflow")
var ErrZeroDivision = errors.New("money: division by zero")
var ErrUnsupportedRounding = errors.New("money: unsupported rounding")

type Micro int64

func (micro Micro) MarshalJSON() ([]byte, error) {
	result := ToString(micro)
	return []byte(result), nil
}

func (micro *Micro) UnmarshalJSON(src []byte) (err error) {
	if src == nil {
		return
	}

	result, err := FromString(strings.Trim(string(src), "\""))
	if err != nil {
		return err
	}
	*micro = result
	return nil
}

func FromString(amount string) (Micro, error) {
	return parseFloatString(amount)
}

func ToString(amount Micro) string {
	decimal := amount / precision
	fraction := amount % precision

	buffer := bytes.Buffer{}

	if fraction < 0 {
		fraction = -fraction

		// we can lose negative sign with division, eg. -999999/1000000 = 0
		if decimal == 0 {
			buffer.WriteString("-")
		}
	}

	buffer.WriteString(strconv.FormatInt(int64(decimal), 10))
	if fraction > 0 {
		buffer.WriteRune('.')
		buffer.WriteString(fmt.Sprintf("%06d", int64(fraction)))
	}

	result := ""
	if fraction > 0 {
		result = strings.TrimRight(buffer.String(), "0")
	} else {
		result = buffer.String()
	}

	return result
}

func FromFloat64(amount float64) (Micro, error) {
	fPrecision := float64(precision)
	if amount > float64(MaxMicro)/fPrecision || amount < float64(MinMicro)/fPrecision {
		return 0, ErrOverflow
	}

	resultFloat := amount * fPrecision
	result := int64(resultFloat)

	return Micro(result), nil
}

func ToFloat64(amount Micro) (float64, error) {
	result := float64(amount) / float64(precision)

	return result, nil
}

func parseFloatString(amount string) (Micro, error) {
	if len(amount) == 0 {
		return Micro(0), ErrInvalidInput
	}

	result := uint64(0)
	sign := int64(1)
	digitsFound := false
	// Significant digit is every digit in integer part after the first non-zero digit or every digit in the decimal part.
	significantDigitFound := false
	dotFound := false
	decimalPartLength := int64(0)

	i := 0
	switch amount[i] {
	case '+':
		i++
	case '-':
		i++
		sign = -1
	}

	for ; i < len(amount); i++ {
		switch c := amount[i]; true {
		case c == '.':
			if dotFound {
				return 0, ErrInvalidInput
			}

			dotFound = true
		case c >= '0' && c <= '9':
			digitsFound = true

			if !significantDigitFound && !dotFound && c == '0' {
				continue
			}
			significantDigitFound = true

			// precisonExp + 1 so that we can do rounding in the end if necessary
			if decimalPartLength == precisionExp+1 {
				continue
			}

			newResult := result * 10
			// overflow
			if result != newResult/10 {
				return 0, ErrOverflow
			}

			newResult += uint64(c - '0')
			// This overflow check is valid because digits can only be 0-9.
			if newResult < result*10 {
				return 0, ErrOverflow
			}

			// In the end, we use signed int64 and this makes sure it doesn't overflow
			if (sign == 1 && newResult > 1<<63-1) || (sign == -1 && newResult > 1<<63) {
				return 0, ErrOverflow
			}

			if dotFound {
				decimalPartLength++
			}

			result = newResult
		default:
			return 0, ErrInvalidInput
		}
	}
	if !digitsFound {
		return 0, ErrInvalidInput
	}

	// If this is true, it can only be precisionExp + 1 decimal places (see how we handle this in switch above)
	if decimalPartLength > precisionExp {
		// rounding
		if result%10 >= 5 {
			newResult := result + 10
			// When rounding, we can be more lax about overflows so just ignore it.
			if newResult > result {
				result = newResult
			}
		}
		result /= 10
	} else {
		for i := int64(0); i < precisionExp-decimalPartLength; i++ {
			newResult := result * 10
			// Overflow
			if result != newResult/10 {
				return 0, ErrOverflow
			}
			result = newResult
		}
	}

	resultSigned := int64(result) * sign

	return Micro(resultSigned), nil
}

func Add(a Micro, b Micro) (Micro, error) {
	result := a + b

	if a < 0 && b < 0 && result >= 0 {
		return 0, ErrOverflow
	}
	if a > 0 && b > 0 && result <= 0 {
		return 0, ErrOverflow
	}
	return result, nil
}

func Mul(amount Micro, multiplier int64) (Micro, error) {
	var mult = Micro(multiplier)
	result := amount * mult

	if mult != 0 && result/mult != amount {
		return 0, ErrOverflow
	}

	return result, nil
}

func divideAndRoundHalfAwayFromZero(a Micro, b Micro) Micro {
	if (a < 0 || b < 0) && !(a < 0 && b < 0) {
		return (a - (b / 2)) / b
	}
	return (a + (b / 2)) / b
}

func Div(amount Micro, divisor int64, rounding byte) (Micro, error) {
	var div = Micro(divisor)
	var result = Micro(0)

	if div == 0 {
		return result, ErrZeroDivision
	}

	switch rounding {
	case RoundingNone:
		result = amount / div
	case RoundingHalfAwayFromZero:
		result = divideAndRoundHalfAwayFromZero(amount, div)
	default:
		return result, ErrUnsupportedRounding
	}

	return result, nil
}
