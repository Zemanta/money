package money

import (
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

type mulTest struct {
	input1   Micro
	input2   int64
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
	{"0.1e10", 0, ErrInvalidInput},
	{"0.1E10", 0, ErrInvalidInput},
	{".1e10", 0, ErrInvalidInput},
	{"1e10", 0, ErrInvalidInput},
	{"100000000000000000000000", 0, ErrOverflow},
	{"1e-100", 0, ErrInvalidInput},
	{"123456700", 123456700 * Dollar, nil},
	{"-1", -1 * Dollar, nil},
	{"-0.1", -10 * Cent, nil},
	{"-0", 0, nil},
	{"1e-20", 0, ErrInvalidInput},
	{"625e-3", 0, ErrInvalidInput},
	{"9223372036854.775807", 9223372036854775807, nil},
	{"-9223372036854.775808", -9223372036854775808, nil},

	// NaNs
	{"nan", 0, ErrInvalidInput},
	{"NaN", 0, ErrInvalidInput},
	{"NAN", 0, ErrInvalidInput},

	// Infs
	{"inf", 0, ErrInvalidInput},
	{"-Inf", 0, ErrInvalidInput},
	{"+INF", 0, ErrInvalidInput},
	{"-Infinity", 0, ErrInvalidInput},
	{"+INFINITY", 0, ErrInvalidInput},
	{"Infinity", 0, ErrInvalidInput},

	// largest money
	{"9000000000.000000", 9000000000000000, nil},
	{"-9000000000.000000", -9000000000000000, nil},
	// too large
	{"9223372036854.775808", 0, ErrOverflow},
	{"-9223372036854.775809", 0, ErrOverflow},

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
	{"9223372036854.775808", 0, ErrOverflow},
	{"-9223372036854.775809", 0, ErrOverflow},
	// a number that overflows but seemingly stops overflowing on the last iteration
	{"92233720368547758087", 0, ErrOverflow},
	// a huge number
	{"100000000000000011102230246251565404236316680908203125" + strings.Repeat("0", 10000) + "1", 0, ErrOverflow},
}

var addTests = []addTest{
	{Micro(0), Micro(0), Micro(0), nil},
	{Micro(0), Micro(1), Micro(1), nil},
	{Micro(1), Micro(0), Micro(1), nil},
	{Micro(0), Micro(-1), Micro(-1), nil},
	{Micro(-1), Micro(0), Micro(-1), nil},

	{Micro(math.MaxInt64), Micro(math.MaxInt64), 0, ErrOverflow},
	{Micro(math.MaxInt64), Micro(1), 0, ErrOverflow},
	{Micro(math.MaxInt64), Micro(0), Micro(math.MaxInt64), nil},

	{Micro(math.MinInt64), Micro(math.MinInt64), 0, ErrOverflow},
	{Micro(math.MinInt64), Micro(-1), 0, ErrOverflow},
	{Micro(math.MinInt64), Micro(0), Micro(math.MinInt64), nil},
}

var mulTests = []mulTest{
	{Micro(0), 0, Micro(0), nil},
	{Micro(0), 1, Micro(0), nil},
	{Micro(1), 0, Micro(0), nil},
	{Micro(0), 1, Micro(0), nil},
	{Micro(-1), 0, Micro(0), nil},
	{Micro(-1), -1, Micro(1), nil},

	{Micro(math.MaxInt64), 0, 0, nil},
	{Micro(math.MaxInt64), 1, Micro(math.MaxInt64), nil},
	{Micro(math.MaxInt64), 2, 0, ErrOverflow},
	{Micro(math.MaxInt64), math.MaxInt64, 0, ErrOverflow},
	{0, math.MaxInt64, 0, nil},

	{Micro(math.MinInt64), 0, 0, nil},
	{Micro(math.MinInt64), 1, Micro(math.MinInt64), nil},
	{Micro(math.MinInt64), 2, 0, ErrOverflow},
	{Micro(math.MinInt64), math.MinInt64, 0, ErrOverflow},
	{0, math.MinInt64, 0, nil},
}

func TestMoneyTestSuite(t *testing.T) {
	suite.Run(t, new(MoneyTestSuite))
}

type MoneyTestSuite struct {
	suite.Suite
}

func (suite *MoneyTestSuite) TestMarshalJSON() {
	var mNil *Micro
	result, err := json.Marshal(mNil)
	suite.Nil(err)
	suite.Equal("null", string(result))

	m := Micro(0)
	result, err = (&m).MarshalJSON()
	suite.Nil(err)
	suite.Equal("0", string(result))

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
	err := (&m).UnmarshalJSON([]byte("9223372036854.775808"))
	suite.Equal(ErrOverflow, err)
	suite.Equal(Micro(0), m)

	m = Micro(10)
	err = (&m).UnmarshalJSON([]byte("-9223372036854.775809"))
	suite.Equal(ErrOverflow, err)
	suite.Equal(Micro(10), m)
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
	suite.Equal(ErrOverflow, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("-9223372036854.775809")
	suite.Equal(ErrOverflow, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("9223372036854.775808")
	suite.Equal(ErrOverflow, err)
	suite.Equal(Micro(0), result)
}

func (suite *MoneyTestSuite) TestInvalidFromFloatStringWithExp() {
	result, err := FromFloatString("123.764538e6")
	suite.Equal(ErrInvalidInput, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("123.764538E6")
	suite.Equal(ErrInvalidInput, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("123.764538e+6")
	suite.Equal(ErrInvalidInput, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("3.7e10")
	suite.Equal(ErrInvalidInput, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("-3.7e10")
	suite.Equal(ErrInvalidInput, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloatString("3.7134234545e10")
	suite.Equal(ErrInvalidInput, err)
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

func (suite *MoneyTestSuite) TestToFloat64() {
	result, err := ToFloat64(123764538)
	suite.Nil(err)
	suite.Equal(123.764538, result)

	result, err = ToFloat64(123)
	suite.Nil(err)
	suite.Equal(0.000123, result)

	result, err = ToFloat64(400)
	suite.Nil(err)
	suite.Equal(0.0004, result)

	result, err = ToFloat64(0)
	suite.Nil(err)
	suite.Equal(0.0, result)

	result, err = ToFloat64(124311)
	suite.Nil(err)
	suite.Equal(0.124311, result)

	result, err = ToFloat64(1204311)
	suite.Nil(err)
	suite.Equal(1.204311, result)

	result, err = ToFloat64(1204311)
	suite.Nil(err)
	suite.Equal(1.204311, result)

	result, err = ToFloat64(10204311)
	suite.Nil(err)
	suite.Equal(10.204311, result)

	result, err = ToFloat64(0)
	suite.Nil(err)
	suite.Equal(float64(0), result)

	result, err = ToFloat64(9000000000000000)
	suite.Nil(err)
	suite.Equal(9000000000.000000, result)
}

func (suite *MoneyTestSuite) TestValidFromFloat64() {
	result, err := FromFloat64(123.764538)
	suite.Nil(err)
	suite.Equal(Micro(123764538), result)

	result, err = FromFloat64(123.52348976)
	suite.Nil(err)
	suite.Equal(Micro(123523489), result)

	result, err = FromFloat64(123.523489)
	suite.Nil(err)
	suite.Equal(Micro(123523489), result)

	result, err = FromFloat64(123)
	suite.Nil(err)
	suite.Equal(Micro(123000000), result)

	result, err = FromFloat64(12.3)
	suite.Nil(err)
	suite.Equal(Micro(12300000), result)
}

func (suite *MoneyTestSuite) TestInvalidFromFloat64() {
	result, err := FromFloat64(13849502840392485906123.764538)
	suite.Equal(ErrOverflow, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloat64(-9223372036854.776808)
	suite.Equal(ErrOverflow, err)
	suite.Equal(Micro(0), result)

	result, err = FromFloat64(9223372036854.776807)
	suite.Equal(ErrOverflow, err)
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

func (suite *MoneyTestSuite) TestAdd() {
	for _, test := range addTests {
		result, err := Add(test.input1, test.input2)
		suite.Equal(test.err, err, fmt.Sprintf("Inputs: %d, %d", test.input1, test.input2))
		suite.Equal(test.expected, result, fmt.Sprintf("Inputs: %d, %d", test.input1, test.input2))
	}
}

func (suite *MoneyTestSuite) TestMul() {
	for _, test := range mulTests {
		result, err := Mul(test.input1, test.input2)
		suite.Equal(test.err, err, fmt.Sprintf("Inputs: %d, %d", test.input1, test.input2))
		suite.Equal(test.expected, result, fmt.Sprintf("Inputs: %d, %d", test.input1, test.input2))
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
