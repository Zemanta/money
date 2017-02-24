package money

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type parseFloatStringTest struct {
	input    string
	expected Micro
	err      error
}

type addTest struct {
	input1   Micro
	input2   Micro
	expected Micro
	err      error
}

var parseFloatStringTests = []parseFloatStringTest{
	{"", Micro(0), ErrInvalidInput},
	{"1", Dollar, nil},
	{"+1", Dollar, nil},
	{"1.1", 110 * Cent, nil},
	{"000001.1", 110 * Cent, nil},
	{"0.", Micro(0), nil},
	{".1", 10 * Cent, nil},
	{"0.1e10", 1000000000 * Dollar, nil},
	{"0.1E10", 1000000000 * Dollar, nil},
	{".1e10", 1000000000 * Dollar, nil},
	{"1e10", Micro(0), ErrOverBounds},
	{"100000000000000000000000", Micro(0), ErrOverBounds},
	{"1e-100", 0, nil},
	{"123456700", 123456700 * Dollar, nil},
	{"-1", -1 * Dollar, nil},
	{"-0.1", -10 * Cent, nil},
	{"-0", 0, nil},
	{"1e-20", 0, nil},
	{"625e-3", 625000, nil},

	// NaNs
	{"nan", 0, ErrOverBounds},
	{"NaN", 0, ErrOverBounds},
	{"NAN", 0, ErrOverBounds},

	// Infs
	{"inf", 0, ErrOverBounds},
	{"-Inf", 0, ErrOverBounds},
	{"+INF", 0, ErrOverBounds},
	{"-Infinity", 0, ErrOverBounds},
	{"+INFINITY", 0, ErrOverBounds},
	{"Infinity", 0, ErrOverBounds},

	// largest money
	{"9000000000.000000", 9000000000000000, nil},
	{"-9000000000.000000", -9000000000000000, nil},
	// too large
	{"9000000000.000001", 0, ErrOverBounds},
	{"-9000000000.000001", 0, ErrOverBounds},
	{"9000000000e1", 0, ErrOverBounds},

	// parse errors
	{"1e", 0, ErrInvalidInput},
	{"1e-", 0, ErrInvalidInput},
	{".e-1", 0, ErrInvalidInput},
	{"1\x00.2", 0, ErrInvalidInput},
	{"1x", Micro(0), ErrInvalidInput},
	{"1.1.", Micro(0), ErrInvalidInput},
	{".e2", 0, ErrInvalidInput},
	{"0.2e", 0, ErrInvalidInput},

	// rounding
	{"22.2222224", 22222222, nil},
	{"22.2222225", 22222223, nil},
	{"22.2222226", 22222223, nil},
	{"-22.2222224", -22222222, nil},
	{"-22.2222225", -22222223, nil},
	{"-22.2222226", -22222223, nil},
	// try to overflow internal result (uint64) with a small number
	{"1.844674407370955161600000000000", 1844674, nil},
	{"-1.844674407370955161700000000000", -1844674, nil},
	// try to overflow internal signed result (int64) with a small number
	{"9.22337203685477580800000000000", 9223372, nil},
	{"-9.2233720368547758080000000000", -9223372, nil},
	{"2.4390000000000000000000000000", 2439000, nil},
	// try with an even smaller but longer number
	{"1.00000000000000011102230246251565404236316680908203125", Dollar, nil},
	{"-1.00000000000000011102230246251565404236316680908203125", -1 * Dollar, nil},
	// max int64 + 1 for overflow
	{"9223372036854.775808", 0, ErrOverBounds},
	{"-9223372036854.775808", 0, ErrOverBounds},
	// a number that overflows but seemingly stops overflowing on the last iteration
	{"92233720368547758087", 0, ErrOverBounds},
	// a huge number
	{"100000000000000011102230246251565404236316680908203125" + strings.Repeat("0", 10000) + "1", 0, ErrOverBounds},

	// try overflown number with exp that produces valid number
	{"9.223372036854775808123e-2", 92234, nil},
	{"9223372036854775808123e-20", 92233720, nil},
	// mantissa overflows but exp makes it valid
	{"9223372036854775808.223372036854775808e-10", 922337203685478, nil},

	{"1e-9223372036854775808", 0, nil},
	// exp is too big and we fall into infinity territory
	{"1e+9223372036854775807", 0, ErrOverBounds},

	// try to overflow exp
	{"1e-9223372036854775809", 0, nil},
	{"1e+9223372036854775808", 0, ErrOverBounds},
	{"1e-18446744073709551616", 0, nil},
	{"1e+18446744073709551616", 0, ErrOverBounds},
}

var addTests = []addTest{
	{Micro(0), Micro(0), Micro(0), nil},
	{Micro(0), Micro(1), Micro(1), nil},
	{Micro(1), Micro(0), Micro(1), nil},

	{Micro(math.MaxInt64), Micro(math.MaxInt64), 0, ErrOverflow},
	{Micro(math.MaxInt64), Micro(1), 0, ErrOverflow},
	{Micro(math.MaxInt64), Micro(0), Micro(math.MaxInt64), nil},

	{Micro(math.MinInt64), Micro(math.MinInt64), 0, ErrOverflow},
	{Micro(math.MinInt64), Micro(-1), 0, ErrOverflow},
	{Micro(math.MinInt64), Micro(0), Micro(math.MinInt64), nil},
}

func TestMoneyTestSuite(t *testing.T) {
	suite.Run(t, new(MoneyTestSuite))
}

type MoneyTestSuite struct {
	suite.Suite
}

func (suite *MoneyTestSuite) TestMarshalJSON() {
	var mNil *Micro
	v, err := json.Marshal(mNil)
	suite.Nil(err)
	suite.Equal([]byte("null"), v)

	m := Micro(0)
	result, err := (&m).MarshalJSON()
	suite.Nil(err)
	suite.Equal([]byte{0x30}, result)

	m = 8 * Dollar
	result, err = (&m).MarshalJSON()
	suite.Nil(err)
	suite.Equal("8", string(result))

	m = 801 * Cent
	result, err = (&m).MarshalJSON()
	suite.Nil(err)
	suite.Equal("8.01", string(result))

	m = 8 * Cent
	result, err = (&m).MarshalJSON()
	suite.Nil(err)
	suite.Equal("0.08", string(result))

	m = -8 * Dollar
	result, err = (&m).MarshalJSON()
	suite.Nil(err)
	suite.Equal("-8", string(result))

	m = -801 * Cent
	result, err = (&m).MarshalJSON()
	suite.Nil(err)
	suite.Equal("-8.01", string(result))
}

func (suite *MoneyTestSuite) TestInvalidMarshalJSON() {
	m := Micro(9000000000000001)
	result, err := (&m).MarshalJSON()
	suite.Equal(ErrOverBounds, err)
	suite.Nil(result)
	suite.Equal(Micro(9000000000000001), m)

	m = Micro(-9000000000000001)
	result, err = (&m).MarshalJSON()
	suite.Equal(ErrOverBounds, err)
	suite.Nil(result)
	suite.Equal(Micro(-9000000000000001), m)
}

func (suite *MoneyTestSuite) TestUnmarshalJSON() {
	var mNil *Micro
	err := mNil.UnmarshalJSON(nil)
	suite.Nil(err)
	suite.Equal((*Micro)(nil), mNil)

	m := Micro(0)
	err = (&m).UnmarshalJSON(nil)
	suite.Nil(err)
	suite.Equal(Micro(0), m)

	m = Micro(100)
	err = (&m).UnmarshalJSON(nil)
	suite.Nil(err)
	suite.Equal(Micro(100), m)

	m = Micro(0)
	err = (&m).UnmarshalJSON([]byte("0.08"))
	suite.Nil(err)
	suite.Equal(8*Cent, m)

	m = Micro(0)
	err = (&m).UnmarshalJSON([]byte("8.01"))
	suite.Nil(err)
	suite.Equal(801*Cent, m)

	m = Micro(0)
	err = (&m).UnmarshalJSON([]byte("8.00"))
	suite.Nil(err)
	suite.Equal(8*Dollar, m)

	m = Micro(0)
	err = (&m).UnmarshalJSON([]byte("8"))
	suite.Nil(err)
	suite.Equal(8*Dollar, m)

	m = Micro(0)
	err = (&m).UnmarshalJSON([]byte("-8"))
	suite.Nil(err)
	suite.Equal(-8*Dollar, m)

	m = Micro(0)
	err = (&m).UnmarshalJSON([]byte("-8.01"))
	suite.Nil(err)
	suite.Equal(-801*Cent, m)
}

func (suite *MoneyTestSuite) TestInvalidUnmarshalJSON() {
	m := Micro(0)
	err := (&m).UnmarshalJSON([]byte("9000000000.01"))
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(0), m)

	m = Micro(10)
	err = (&m).UnmarshalJSON([]byte("-9000000000.01"))
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(10), m)
}

func (suite *MoneyTestSuite) TestScan() {
	var mNil *Micro
	err := mNil.Scan(nil)
	suite.Nil(err)
	suite.Equal((*Micro)(nil), mNil)

	m := Micro(0)
	err = (&m).Scan([]uint8{48, 46, 48, 56, 48, 48, 48, 48})
	suite.Nil(err)
	suite.Equal(Micro(80000), m)

	err = (&m).Scan([]uint8{48, 46, 51, 53, 48, 48, 48, 48})
	suite.Nil(err)
	suite.Equal(Micro(350000), m)

	err = (&m).Scan([]uint8{49, 46, 50, 53, 48, 48, 48, 48})
	suite.Nil(err)
	suite.Equal(Micro(1250000), m)

	err = (&m).Scan([]uint8{49, 48, 48, 48, 46, 48, 48, 48, 48, 48, 48})
	suite.Nil(err)
	suite.Equal(Micro(1000000000), m)

	err = (&m).Scan([]uint8{45, 49, 46, 48, 48, 48, 48, 48, 48})
	suite.Nil(err)
	suite.Equal(Micro(-1000000), m)
}

func (suite *MoneyTestSuite) TestInvalidScan() {
	m := Micro(100)
	// 9000000000.000001
	err := (&m).Scan([]uint8{57, 48, 48, 48, 48, 48, 48, 48, 48, 48, 46, 48, 48, 48, 48, 48, 49})
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(100), m)

	// -9000000000.000001
	err = (&m).Scan([]uint8{45, 57, 48, 48, 48, 48, 48, 48, 48, 48, 48, 46, 48, 48, 48, 48, 48, 49})
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(100), m)
}

func (suite *MoneyTestSuite) TestValue() {
	m := new(Micro)
	*m = 12 * Cent
	val, err := m.Value()
	suite.Nil(err)
	suite.Equal("0.12", val)

	*m = 125 * Cent
	val, err = m.Value()
	suite.Nil(err)
	suite.Equal("1.25", val)

	*m = 0 * Cent
	val, err = m.Value()
	suite.Nil(err)
	suite.Equal("0", val)

	mv := 12 * Cent
	val, err = mv.Value()
	suite.Nil(err)
	suite.Equal("0.12", val)

	_ = driver.Valuer(mv)
}

func (suite *MoneyTestSuite) TestInvalidValue() {
	m := new(Micro)
	*m = 9000000000000001
	val, err := m.Value()
	suite.Equal(ErrOverBounds, err)
	suite.Equal(nil, val)

	*m = -9000000000000001
	val, err = m.Value()
	suite.Equal(ErrOverBounds, err)
	suite.Equal(nil, val)
}

func (suite *MoneyTestSuite) TestValidFromFloatString() {
	result, err := FromFloatString("123.764538")
	suite.Nil(err)
	suite.Equal(Micro(123764538), result)

	result, err = FromFloatString("123.52348976")
	suite.Nil(err)
	suite.Equal(Micro(123523490), result)

	result, err = FromFloatString("0000123.523489")
	suite.Nil(err)
	suite.Equal(Micro(123523489), result)

	result, err = FromFloatString("123")
	suite.Nil(err)
	suite.Equal(Micro(123000000), result)

	result, err = FromFloatString("12.3")
	suite.Nil(err)
	suite.Equal(Micro(12300000), result)

	result, err = FromFloatString("-12")
	suite.Nil(err)
	suite.Equal(Micro(-12000000), result)

	result, err = FromFloatString("-12.3")
	suite.Nil(err)
	suite.Equal(Micro(-12300000), result)
}

func (suite *MoneyTestSuite) TestInvalidFromFloatString() {
	result, err := FromFloatString("123.764.538")
	suite.Equal(ErrInvalidInput, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("123.7 64538")
	suite.Equal(ErrInvalidInput, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("a123.764538")
	suite.Equal(ErrInvalidInput, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("13849502840392485906123.764538")
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("-9000000000.000001")
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("9000000000.000001")
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(0), result)
}

func (suite *MoneyTestSuite) TestValidFromFloatStringWithExp() {
	result, err := FromFloatString("123.764538e6")
	suite.Nil(err)
	suite.Equal(Micro(123764538000000), result)

	result, err = FromFloatString("123.764538E6")
	suite.Nil(err)
	suite.Equal(Micro(123764538000000), result)

	result, err = FromFloatString("123.764538e+6")
	suite.Nil(err)
	suite.Equal(Micro(123764538000000), result)

	result, err = FromFloatString("3.7134234545e9")
	suite.Nil(err)
	suite.Equal(Micro(3713423454500000), result)

	result, err = FromFloatString("0.725344545454654645e10")
	suite.Nil(err)
	suite.Equal(Micro(7253445454546546), result)

	result, err = FromFloatString("123.764538e-6")
	suite.Nil(err)
	suite.Equal(Micro(124), result)

	result, err = FromFloatString("12345.7e-10")
	suite.Nil(err)
	suite.Equal(Micro(1), result)
}

func (suite *MoneyTestSuite) TestInvalidFromFloatStringWithExp() {
	result, err := FromFloatString("3.7e10")
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("-3.7e10")
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("3.7134234545e10")
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(0), result)
}

func (suite *MoneyTestSuite) TestValidToFloatString() {
	result, err := ToFloatString(123764538)
	suite.Nil(err)
	suite.Equal("123.764538", result)

	result, err = ToFloatString(-999999)
	suite.Nil(err)
	suite.Equal("-0.999999", result)

	result, err = ToFloatString(12352348976)
	suite.Nil(err)
	suite.Equal("12352.348976", result)

	result, err = ToFloatString(123523489000)
	suite.Nil(err)
	suite.Equal("123523.489", result)

	result, err = ToFloatString(123)
	suite.Nil(err)
	suite.Equal("0.000123", result)

	result, err = ToFloatString(0)
	suite.Nil(err)
	suite.Equal("0", result)

	result, err = ToFloatString(-123000000)
	suite.Nil(err)
	suite.Equal("-123", result)

	result, err = ToFloatString(-123764538)
	suite.Nil(err)
	suite.Equal("-123.764538", result)
}

func (suite *MoneyTestSuite) TestInvalidToFloatString() {
	result, err := ToFloatString(Micro(9000000000000001))
	suite.Equal(ErrOverBounds, err)
	suite.Equal("", result)

	result, err = ToFloatString(Micro(-9000000000000001))
	suite.Equal(ErrOverBounds, err)
	suite.Equal("", result)
}

func (suite *MoneyTestSuite) TestToFloat64Dollar() {
	result, err := ToFloat64Dollar(123764538)
	suite.Nil(err)
	suite.Equal(123.764538, result)

	result, err = ToFloat64Dollar(123)
	suite.Nil(err)
	suite.Equal(0.000123, result)

	result, err = ToFloat64Dollar(400)
	suite.Nil(err)
	suite.Equal(0.0004, result)

	result, err = ToFloat64Dollar(0)
	suite.Nil(err)
	suite.Equal(0.0, result)

	result, err = ToFloat64Dollar(124311)
	suite.Nil(err)
	suite.Equal(0.124311, result)

	result, err = ToFloat64Dollar(1204311)
	suite.Nil(err)
	suite.Equal(1.204311, result)

	result, err = ToFloat64Dollar(1204311)
	suite.Nil(err)
	suite.Equal(1.204311, result)

	result, err = ToFloat64Dollar(10204311)
	suite.Nil(err)
	suite.Equal(10.204311, result)

	result, err = ToFloat64Dollar(0)
	suite.Nil(err)
	suite.Equal(float64(0), result)

	result, err = ToFloat64Dollar(9000000000000000)
	suite.Nil(err)
	suite.Equal(9000000000.000000, result)
}

func (suite *MoneyTestSuite) TestInvalidToFloat64Dollar() {
	result, err := ToFloat64Dollar(-9000000000000001)
	suite.Equal(ErrOverBounds, err)
	suite.Equal(float64(0), result)

	result, err = ToFloat64Dollar(9000000000000001)
	suite.Equal(ErrOverBounds, err)
	suite.Equal(float64(0), result)
}

func (suite *MoneyTestSuite) TestValidFromFloat64Dollar() {
	result, err := FromFloat64Dollar(123.764538)
	suite.Nil(err)
	suite.Equal(Micro(123764538), result)

	result, err = FromFloat64Dollar(123.52348976)
	suite.Nil(err)
	suite.Equal(Micro(123523489), result)

	result, err = FromFloat64Dollar(123.523489)
	suite.Nil(err)
	suite.Equal(Micro(123523489), result)

	result, err = FromFloat64Dollar(123)
	suite.Nil(err)
	suite.Equal(Micro(123000000), result)

	result, err = FromFloat64Dollar(12.3)
	suite.Nil(err)
	suite.Equal(Micro(12300000), result)
}

func (suite *MoneyTestSuite) TestInvalidFromFloat64Dollar() {
	result, err := FromFloat64Dollar(13849502840392485906123.764538)
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloat64Dollar(-9000000000.000001)
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloat64Dollar(9000000000.000001)
	suite.Equal(ErrOverBounds, err)
	suite.Equal(Micro(0), result)
}

func (suite *MoneyTestSuite) TestDivideAndRound() {
	suite.Equal(DivideAndRound(1499, 1000), Micro(1))
	suite.Equal(DivideAndRound(1500, 1000), Micro(2))

	suite.Equal(DivideAndRound(10, 7), Micro(1))
	suite.Equal(DivideAndRound(11, 7), Micro(2))

	suite.Equal(DivideAndRound(11, 2), Micro(6))
	suite.Equal(DivideAndRound(12, 2), Micro(6))

	suite.Equal(DivideAndRound(-1499, 1000), Micro(-1))
	suite.Equal(DivideAndRound(-1500, 1000), Micro(-2))

	suite.Equal(DivideAndRound(-10, 7), Micro(-1))
	suite.Equal(DivideAndRound(-11, 7), Micro(-2))

	suite.Equal(DivideAndRound(-11, 2), Micro(-6))
	suite.Equal(DivideAndRound(-12, 2), Micro(-6))

	suite.Equal(DivideAndRound(1499, -1000), Micro(-1))
	suite.Equal(DivideAndRound(1500, -1000), Micro(-2))

	suite.Equal(DivideAndRound(10, -7), Micro(-1))
	suite.Equal(DivideAndRound(11, -7), Micro(-2))

	suite.Equal(DivideAndRound(11, -2), Micro(-6))
	suite.Equal(DivideAndRound(12, -2), Micro(-6))

	suite.Equal(DivideAndRound(-1499, -1000), Micro(1))
	suite.Equal(DivideAndRound(-1500, -1000), Micro(2))

	suite.Equal(DivideAndRound(-10, -7), Micro(1))
	suite.Equal(DivideAndRound(-11, -7), Micro(2))

	suite.Equal(DivideAndRound(-11, -2), Micro(6))
	suite.Equal(DivideAndRound(-12, -2), Micro(6))
}

func (suite *MoneyTestSuite) TestParseFloatString() {
	for _, test := range parseFloatStringTests {
		result, err := parseFloatString(test.input)
		suite.Equal(test.err, err, fmt.Sprintf("Input: %s", test.input))
		suite.Equal(test.expected, result, fmt.Sprintf("Input: %s", test.input))
	}
}

func BenchmarkFromFloatString(b *testing.B) {
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, err := FromFloatString("123.52348976")
		if err != nil {
			b.Error(errors.New("Unsuccessful call."))
		}
	}
}

func BenchmarkFromFloatStringWithExp(b *testing.B) {
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, err := FromFloatString("123.52348976e-2")
		if err != nil {
			b.Error(errors.New("Unsuccessful call."))
		}
	}
}

func BenchmarkParseFloatString(b *testing.B) {
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, err := parseFloatString("123.52348976")
		if err != nil {
			b.Error(errors.New("Unsuccessful call."))
		}
	}
}

func (suite *MoneyTestSuite) TestAdd() {
	for _, test := range addTests {
		result, err := Add(test.input1, test.input2)
		suite.Equal(test.err, err, fmt.Sprintf("Inputs: %d, %d", test.input1, test.input2))
		suite.Equal(test.expected, result, fmt.Sprintf("Inputs: %d, %d", test.input1, test.input2))
	}
}
