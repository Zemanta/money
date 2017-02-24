package money

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const (
	precisionExp         = int64(6)
	precision            = Micro(1000000)
	largestAmount        = int64(9000000000000000)
	smallestAmount       = int64(-9000000000000000)
	Zero                 = Micro(0)
	MicroDollar    Micro = 1
	Cent                 = 10000 * MicroDollar
	Dollar               = 100 * Cent
)

var precisionRat = big.NewRat(int64(precision), 1)

var ErrOverBounds = errors.New("Amount for money.Micro has to be larger than or equal to -9000000000000000 and less than or equal to 9000000000000000")
var ErrInvalidInput = errors.New("Cannot convert string to money.Micro.")

type Micro int64

func (micro Micro) MarshalJSON() ([]byte, error) {
	result, err := ToFloatString(micro)
	if err != nil {
		return nil, err
	}
	return []byte(result), nil
}

func (micro *Micro) UnmarshalJSON(src []byte) (err error) {
	if src == nil {
		return
	}

	result, err := FromFloatString(strings.Trim(string(src), "\""))
	if err != nil {
		return err
	}
	*micro = result
	return nil
}

func (micro *Micro) Scan(src interface{}) (err error) {
	if src == nil {
		return
	}

	result, err := FromFloatString(string(src.([]uint8)))
	if err != nil {
		return err
	}

	*micro = result

	return nil
}

func (micro Micro) Value() (driver.Value, error) {
	val, err := ToFloatString(micro)
	if err != nil {
		return nil, err
	}

	return val, nil
}

func FromFloatString(amount string) (Micro, error) {
	return parseFloatString(amount)
}

func ToFloatString(amount Micro) (string, error) {
	if err := checkNumberBounds(int64(amount)); err != nil {
		return "", err
	}

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

	return result, nil
}

func FromBigRat(amount *big.Rat) (Micro, error) {
	// We need 7 decimal digits because of rounding.
	result, err := FromFloatString(amount.FloatString(7))
	if err != nil {
		return 0, err
	}

	return result, nil
}

func FromFloat64Dollar(amount float64) (Micro, error) {
	resultFloat := amount * float64(precision)
	result := int64(resultFloat)
	if err := checkNumberBounds(result); err != nil {
		return 0, err
	}

	return Micro(result), nil
}

func ToFloat64Dollar(amount Micro) (float64, error) {
	if err := checkNumberBounds(int64(amount)); err != nil {
		return 0, err
	}
	result := float64(amount) / float64(precision)

	return result, nil
}

func DivideAndRound(a Micro, b int64) Micro {
	if (a < 0 || b < 0) && !(a < 0 && b < 0) {
		return (a - (Micro(b) / 2)) / Micro(b)
	}
	return (a + (Micro(b) / 2)) / Micro(b)
}

func checkNumberBounds(amount int64) error {
	if amount < smallestAmount || amount > largestAmount {
		return ErrOverBounds
	}

	return nil
}

func parseFloatString(amount string) (Micro, error) {
	if len(amount) == 0 {
		return 0, ErrInvalidInput
	}
	result, err := parseFloatStringInt(amount)
	if err != nil {
		result, err = parseFloatStringFloat(amount)
	}

	return result, err
}

func parseFloatStringFloat(amount string) (Micro, error) {
	result, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		if errNumError, ok := err.(*strconv.NumError); ok {
			if errNumError.Err == strconv.ErrRange {
				return 0, ErrOverBounds
			} else {
				return 0, ErrInvalidInput
			}
		}
		return 0, err
	}
	result = result * float64(precision)
	// Rounding
	if result < 0 {
		result -= 0.5
	} else {
		result += 0.5
	}
	resultInt := int64(result)
	if err := checkNumberBounds(resultInt); err != nil {
		return 0, err
	}
	return Micro(resultInt), nil
}

func parseFloatStringInt(amount string) (Micro, error) {
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
				return 0, ErrOverBounds
			}

			newResult += uint64(c - '0')
			// This overflow check is valid because digits can only be 0-9.
			if newResult < result*10 {
				return 0, ErrOverBounds
			}

			// In the end, we use signed int64 and this makes sure it doesn't overflow
			if (sign == 1 && newResult > 1<<63-1) || (sign == -1 && newResult > 1<<63) {
				return 0, ErrOverBounds
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
				return 0, ErrOverBounds
			}
			result = newResult
		}
	}

	resultSigned := int64(result) * sign
	if err := checkNumberBounds(resultSigned); err != nil {
		return 0, err
	}

	return Micro(resultSigned), nil
}
