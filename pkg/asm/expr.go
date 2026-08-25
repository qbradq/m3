package asm

import (
	"fmt"
)

type Expr interface {
	Pos() Position
	Eval(lookup SymbolResolver) (int64, error)
	String() string
}

type SymbolResolver interface {
	LookupSymbol(name string) (val int64, bank int, found bool)
}

type MapSymbolResolver struct {
	Symbols map[string]int64
	Banks   map[string]int
}

func (m *MapSymbolResolver) LookupSymbol(name string) (int64, int, bool) {
	val, ok := m.Symbols[name]
	if !ok {
		return 0, -1, false
	}
	bank := -1
	if m.Banks != nil {
		if b, hasBank := m.Banks[name]; hasBank {
			bank = b
		}
	}
	return val, bank, true
}

// Number Literal
type NumberExpr struct {
	Value int64
	pos   Position
}

func (n *NumberExpr) Pos() Position { return n.pos }
func (n *NumberExpr) Eval(r SymbolResolver) (int64, error) {
	return n.Value, nil
}
func (n *NumberExpr) String() string {
	return fmt.Sprintf("%d", n.Value)
}

// Symbol / Identifier Reference
type SymbolExpr struct {
	Name string
	pos  Position
}

func (s *SymbolExpr) Pos() Position { return s.pos }
func (s *SymbolExpr) Eval(r SymbolResolver) (int64, error) {
	if r == nil {
		return 0, fmt.Errorf("%s: unresolved symbol %q", s.pos, s.Name)
	}
	val, _, found := r.LookupSymbol(s.Name)
	if !found {
		return 0, fmt.Errorf("%s: undefined symbol %q", s.pos, s.Name)
	}
	return val, nil
}
func (s *SymbolExpr) String() string {
	return s.Name
}

// Unary Expression
type UnaryOp int

const (
	UnaryPos UnaryOp = iota
	UnaryNeg
	UnaryBitNot
	UnaryLogNot
	UnaryLowByte  // <
	UnaryHighByte // >
	UnaryBankByte // ^
	UnaryZeroPage // z:
	UnaryAbsolute // a:
)

type UnaryExpr struct {
	Op    UnaryOp
	Right Expr
	pos   Position
}

func (u *UnaryExpr) Pos() Position { return u.pos }
func (u *UnaryExpr) Eval(r SymbolResolver) (int64, error) {
	if u.Op == UnaryBankByte {
		// If right is a symbol, we can try to look up its bank directly
		if sym, ok := u.Right.(*SymbolExpr); ok && r != nil {
			_, bank, found := r.LookupSymbol(sym.Name)
			if found && bank >= 0 {
				return int64(bank), nil
			}
		}
	}

	val, err := u.Right.Eval(r)
	if err != nil {
		return 0, err
	}

	switch u.Op {
	case UnaryPos:
		return val, nil
	case UnaryNeg:
		return -val, nil
	case UnaryBitNot:
		return ^val, nil
	case UnaryLogNot:
		if val == 0 {
			return 1, nil
		}
		return 0, nil
	case UnaryLowByte:
		return val & 0xFF, nil
	case UnaryHighByte:
		return (val >> 8) & 0xFF, nil
	case UnaryBankByte:
		return (val >> 16) & 0xFF, nil
	case UnaryZeroPage, UnaryAbsolute:
		return val, nil
	default:
		return 0, fmt.Errorf("%s: unknown unary operator %d", u.pos, u.Op)
	}
}

func (u *UnaryExpr) String() string {
	switch u.Op {
	case UnaryPos:
		return "+" + u.Right.String()
	case UnaryNeg:
		return "-" + u.Right.String()
	case UnaryBitNot:
		return "~" + u.Right.String()
	case UnaryLogNot:
		return "!" + u.Right.String()
	case UnaryLowByte:
		return "<" + u.Right.String()
	case UnaryHighByte:
		return ">" + u.Right.String()
	case UnaryBankByte:
		return "^" + u.Right.String()
	case UnaryZeroPage:
		return "z:" + u.Right.String()
	case UnaryAbsolute:
		return "a:" + u.Right.String()
	default:
		return "?" + u.Right.String()
	}
}

// Binary Expression
type BinaryOp int

const (
	BinAdd BinaryOp = iota
	BinSub
	BinMul
	BinDiv
	BinMod
	BinShl
	BinShr
	BinLt
	BinLtEq
	BinGt
	BinGtEq
	BinEq
	BinNotEq
	BinBitAnd
	BinBitXor
	BinBitOr
	BinLogAnd
	BinLogOr
)

type BinaryExpr struct {
	Op    BinaryOp
	Left  Expr
	Right Expr
	pos   Position
}

func (b *BinaryExpr) Pos() Position { return b.pos }
func (b *BinaryExpr) Eval(r SymbolResolver) (int64, error) {
	lVal, err := b.Left.Eval(r)
	if err != nil {
		return 0, err
	}
	rVal, err := b.Right.Eval(r)
	if err != nil {
		return 0, err
	}

	switch b.Op {
	case BinAdd:
		return lVal + rVal, nil
	case BinSub:
		return lVal - rVal, nil
	case BinMul:
		return lVal * rVal, nil
	case BinDiv:
		if rVal == 0 {
			return 0, fmt.Errorf("%s: division by zero", b.pos)
		}
		return lVal / rVal, nil
	case BinMod:
		if rVal == 0 {
			return 0, fmt.Errorf("%s: modulo by zero", b.pos)
		}
		return lVal % rVal, nil
	case BinShl:
		return lVal << uint(rVal), nil
	case BinShr:
		return int64(uint64(lVal) >> uint(rVal)), nil
	case BinLt:
		if lVal < rVal {
			return 1, nil
		}
		return 0, nil
	case BinLtEq:
		if lVal <= rVal {
			return 1, nil
		}
		return 0, nil
	case BinGt:
		if lVal > rVal {
			return 1, nil
		}
		return 0, nil
	case BinGtEq:
		if lVal >= rVal {
			return 1, nil
		}
		return 0, nil
	case BinEq:
		if lVal == rVal {
			return 1, nil
		}
		return 0, nil
	case BinNotEq:
		if lVal != rVal {
			return 1, nil
		}
		return 0, nil
	case BinBitAnd:
		return lVal & rVal, nil
	case BinBitXor:
		return lVal ^ rVal, nil
	case BinBitOr:
		return lVal | rVal, nil
	case BinLogAnd:
		if lVal != 0 && rVal != 0 {
			return 1, nil
		}
		return 0, nil
	case BinLogOr:
		if lVal != 0 || rVal != 0 {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("%s: unknown binary operator %d", b.pos, b.Op)
	}
}

func (b *BinaryExpr) String() string {
	return fmt.Sprintf("(%s %s %s)", b.Left.String(), binOpString(b.Op), b.Right.String())
}

func binOpString(op BinaryOp) string {
	switch op {
	case BinAdd:
		return "+"
	case BinSub:
		return "-"
	case BinMul:
		return "*"
	case BinDiv:
		return "/"
	case BinMod:
		return "%"
	case BinShl:
		return "<<"
	case BinShr:
		return ">>"
	case BinLt:
		return "<"
	case BinLtEq:
		return "<="
	case BinGt:
		return ">"
	case BinGtEq:
		return ">="
	case BinEq:
		return "=="
	case BinNotEq:
		return "!="
	case BinBitAnd:
		return "&"
	case BinBitXor:
		return "^"
	case BinBitOr:
		return "|"
	case BinLogAnd:
		return "&&"
	case BinLogOr:
		return "||"
	default:
		return "?"
	}
}
