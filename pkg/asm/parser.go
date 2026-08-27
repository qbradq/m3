package asm

import (
	"fmt"
	"strings"

	"github.com/qbradq/m3/pkg/cpu6502"
)

type Parser struct {
	lexer   *Lexer
	curTok  Token
	peekTok Token
	errors  []string
}

func NewParser(lexer *Lexer) (*Parser, error) {
	p := &Parser{lexer: lexer}
	// Prime the two tokens
	var err error
	p.curTok, err = p.lexer.NextToken()
	if err != nil {
		return nil, err
	}
	p.peekTok, err = p.lexer.NextToken()
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Parser) nextToken() error {
	p.curTok = p.peekTok
	var err error
	p.peekTok, err = p.lexer.NextToken()
	return err
}

func (p *Parser) curTokenIs(t TokenType) bool {
	return p.curTok.Type == t
}

func (p *Parser) peekTokenIs(t TokenType) bool {
	return p.peekTok.Type == t
}

func (p *Parser) expect(t TokenType) error {
	if !p.curTokenIs(t) {
		return fmt.Errorf("%s: expected token %d, got %v (%q)", p.curTok.Pos, t, p.curTok.Type, p.curTok.Literal)
	}
	return p.nextToken()
}

func (p *Parser) skipNewlines() error {
	for p.curTokenIs(TokenEOL) {
		if err := p.nextToken(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) ParseProgram() ([]Statement, error) {
	var stmts []Statement
	for !p.curTokenIs(TokenEOF) {
		if p.curTokenIs(TokenEOL) {
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			continue
		}

		lineStmts, err := p.parseLine()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, lineStmts...)

		// After parsing a line, consume EOL or expect EOF
		if p.curTokenIs(TokenEOL) {
			if err := p.nextToken(); err != nil {
				return nil, err
			}
		} else if !p.curTokenIs(TokenEOF) {
			return nil, fmt.Errorf("%s: unexpected token at end of statement: %v (%q)", p.curTok.Pos, p.curTok.Type, p.curTok.Literal)
		}
	}
	return stmts, nil
}

func (p *Parser) parseLine() ([]Statement, error) {
	var stmts []Statement

	// 1. Check for Anonymous Label ':'
	if p.curTokenIs(TokenColon) {
		pos := p.curTok.Pos
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		stmts = append(stmts, &LabelStmt{
			Name:   ":",
			IsAnon: true,
			pos:    pos,
		})
		// If line ends after label, return
		if p.curTokenIs(TokenEOL) || p.curTokenIs(TokenEOF) {
			return stmts, nil
		}
	} else if p.curTokenIs(TokenLocalIdent) {
		// 2. Local Label '@name:' or '@name'
		pos := p.curTok.Pos
		name := p.curTok.Literal
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		if p.curTokenIs(TokenColon) {
			if err := p.nextToken(); err != nil {
				return nil, err
			}
		}
		stmts = append(stmts, &LabelStmt{
			Name:    name,
			IsLocal: true,
			pos:     pos,
		})
		if p.curTokenIs(TokenEOL) || p.curTokenIs(TokenEOF) {
			return stmts, nil
		}
	} else if p.curTokenIs(TokenIdent) && (p.peekTokenIs(TokenColon) || (!cpu6502.IsMnemonic(p.curTok.Literal) && (p.peekTokenIs(TokenAssign) || p.peekTokenIs(TokenDotSet) || p.peekTokenIs(TokenDotEqu) || p.peekTokenIs(TokenDotDefine)))) {
		// 3. Global Label 'ident:' or Assignment 'ident = expr' or 'ident .define expr'
		pos := p.curTok.Pos
		ident := p.curTok.Literal
		if err := p.nextToken(); err != nil {
			return nil, err
		}

		if p.curTokenIs(TokenColon) {
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			stmts = append(stmts, &LabelStmt{
				Name: ident,
				pos:  pos,
			})
			if p.curTokenIs(TokenEOL) || p.curTokenIs(TokenEOF) {
				return stmts, nil
			}
		} else if p.curTokenIs(TokenDotDefine) {
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			if p.curTokenIs(TokenAssign) || p.curTokenIs(TokenComma) {
				if err := p.nextToken(); err != nil {
					return nil, err
				}
			}
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, &DefineDirective{
				Name:  ident,
				Value: expr,
				pos:   pos,
			})
			return stmts, nil
		} else if p.curTokenIs(TokenAssign) || p.curTokenIs(TokenDotSet) || p.curTokenIs(TokenDotEqu) {
			isSet := p.curTokenIs(TokenDotSet)
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, &AssignmentStmt{
				Name:  ident,
				Value: expr,
				IsSet: isSet,
				pos:   pos,
			})
			return stmts, nil
		}
	}

	// Now parse directive or instruction if present
	if p.curTokenIs(TokenEOL) || p.curTokenIs(TokenEOF) {
		return stmts, nil
	}

	stmt, err := p.parseOperationOrDirective()
	if err != nil {
		return nil, err
	}
	if stmt != nil {
		stmts = append(stmts, stmt)
	}

	return stmts, nil
}

func (p *Parser) parseOperationOrDirective() (Statement, error) {
	// Directives
	switch p.curTok.Type {
	case TokenDotDefine:
		return p.parseDefineDirective()
	case TokenDotBank:
		return p.parseBankDirective()
	case TokenDotZP:
		return p.parseMemoryDirective(SegmentZP)
	case TokenDotRAM:
		return p.parseMemoryDirective(SegmentRAM)
	case TokenDotWRAM:
		return p.parseMemoryDirective(SegmentWRAM)
	case TokenDotData:
		return p.parseMemoryDirective(SegmentData)
	case TokenDotCode:
		return p.parseMemoryDirective(SegmentPRG)
	case TokenDotByte:
		return p.parseDataDirective(DataByte)
	case TokenDotWord:
		return p.parseDataDirective(DataWord)
	case TokenDotDword:
		return p.parseDataDirective(DataDword)
	case TokenDotAsciiz:
		return p.parseDataDirective(DataAsciiz)
	case TokenDotRes:
		return p.parseReserveDirective()
	case TokenDotExport:
		return p.parseExportDirective()
	case TokenDotImport, TokenDotImportZP:
		return p.parseImportDirective(p.curTok.Type == TokenDotImportZP)
	case TokenDotProc, TokenDotScope:
		return p.parseScopeDirective(p.curTok.Type == TokenDotProc, false)
	case TokenDotEndProc, TokenDotEndScope:
		return p.parseScopeDirective(p.curTok.Type == TokenDotEndProc, true)
	case TokenDotInclude, TokenDotIncbin:
		return p.parseIncludeDirective(p.curTok.Type == TokenDotIncbin)
	case TokenDotIncchr:
		return p.parseIncchrDirective()
	case TokenDotIncpal:
		return p.parseIncpalDirective()
	}

	// Instructions
	if (p.curTokenIs(TokenIdent) && cpu6502.IsMnemonic(p.curTok.Literal)) || p.curTokenIs(TokenRegA) {
		return p.parseInstruction()
	}

	return nil, fmt.Errorf("%s: unrecognized statement or syntax '%s'", p.curTok.Pos, p.curTok.Literal)
}

func (p *Parser) parseDefineDirective() (Statement, error) {
	pos := p.curTok.Pos
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	if !p.curTokenIs(TokenIdent) {
		return nil, fmt.Errorf("%s: expected identifier after .define", p.curTok.Pos)
	}
	name := p.curTok.Literal
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	if p.curTokenIs(TokenAssign) || p.curTokenIs(TokenComma) {
		if err := p.nextToken(); err != nil {
			return nil, err
		}
	}
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &DefineDirective{Name: name, Value: expr, pos: pos}, nil
}

func (p *Parser) parseBankDirective() (Statement, error) {
	pos := p.curTok.Pos
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	if p.curTokenIs(TokenIdent) && strings.ToLower(p.curTok.Literal) == "auto" {
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		return &BankDirective{IsAuto: true, pos: pos}, nil
	}
	bankExpr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &BankDirective{BankIndex: bankExpr, pos: pos}, nil
}

func (p *Parser) parseMemoryDirective(seg MemorySegment) (Statement, error) {
	pos := p.curTok.Pos
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	if !p.curTokenIs(TokenEOL) && !p.curTokenIs(TokenEOF) && !p.curTokenIs(TokenComment) {
		sizeExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		return &MemoryDirective{Segment: seg, Size: sizeExpr, pos: pos}, nil
	}
	return &MemoryDirective{Segment: seg, Size: nil, pos: pos}, nil
}

func (p *Parser) parseDataDirective(dType DataType) (Statement, error) {
	pos := p.curTok.Pos
	if err := p.nextToken(); err != nil {
		return nil, err
	}

	var items []DataItem
	for {
		if p.curTokenIs(TokenString) {
			items = append(items, DataItem{
				String: p.curTok.Literal,
				IsStr:  true,
			})
			if err := p.nextToken(); err != nil {
				return nil, err
			}
		} else {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			items = append(items, DataItem{
				Expr:  expr,
				IsStr: false,
			})
		}

		if p.curTokenIs(TokenComma) {
			if err := p.nextToken(); err != nil {
				return nil, err
			}
		} else {
			break
		}
	}
	return &DataDirective{Type: dType, Items: items, pos: pos}, nil
}

func (p *Parser) parseReserveDirective() (Statement, error) {
	pos := p.curTok.Pos
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	sizeExpr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	var fillExpr Expr
	if p.curTokenIs(TokenComma) {
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		fillExpr, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	return &ReserveDirective{Size: sizeExpr, Fill: fillExpr, pos: pos}, nil
}

func (p *Parser) parseExportDirective() (Statement, error) {
	pos := p.curTok.Pos
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	var names []string
	for {
		if !p.curTokenIs(TokenIdent) {
			return nil, fmt.Errorf("%s: expected identifier in .export directive", p.curTok.Pos)
		}
		names = append(names, p.curTok.Literal)
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		if p.curTokenIs(TokenComma) {
			if err := p.nextToken(); err != nil {
				return nil, err
			}
		} else {
			break
		}
	}
	return &ExportDirective{Names: names, pos: pos}, nil
}

func (p *Parser) parseImportDirective(isZP bool) (Statement, error) {
	pos := p.curTok.Pos
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	var names []string
	for {
		if !p.curTokenIs(TokenIdent) {
			return nil, fmt.Errorf("%s: expected identifier in .import directive", p.curTok.Pos)
		}
		names = append(names, p.curTok.Literal)
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		if p.curTokenIs(TokenComma) {
			if err := p.nextToken(); err != nil {
				return nil, err
			}
		} else {
			break
		}
	}
	return &ImportDirective{Names: names, IsZeroPage: isZP, pos: pos}, nil
}

func (p *Parser) parseScopeDirective(isProc, isEnd bool) (Statement, error) {
	pos := p.curTok.Pos
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	var name string
	if !isEnd && p.curTokenIs(TokenIdent) {
		name = p.curTok.Literal
		if err := p.nextToken(); err != nil {
			return nil, err
		}
	}
	return &ScopeDirective{Name: name, IsProc: isProc, IsEnd: isEnd, pos: pos}, nil
}

func (p *Parser) parseIncludeDirective(isBin bool) (Statement, error) {
	pos := p.curTok.Pos
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	if !p.curTokenIs(TokenString) {
		return nil, fmt.Errorf("%s: expected file path string literal", p.curTok.Pos)
	}
	filename := p.curTok.Literal
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	var offset, size Expr
	if isBin && p.curTokenIs(TokenComma) {
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		var err error
		offset, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.curTokenIs(TokenComma) {
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			size, err = p.parseExpression()
			if err != nil {
				return nil, err
			}
		}
	}
	return &IncludeDirective{Filename: filename, IsBin: isBin, Offset: offset, Size: size, pos: pos}, nil
}

func (p *Parser) parseIncchrDirective() (Statement, error) {
	pos := p.curTok.Pos
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	if !p.curTokenIs(TokenString) {
		return nil, fmt.Errorf("%s: expected PNG file path string literal in .incchr directive", p.curTok.Pos)
	}
	filename := p.curTok.Literal
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	return &IncchrDirective{Filename: filename, pos: pos}, nil
}

func (p *Parser) parseIncpalDirective() (Statement, error) {
	pos := p.curTok.Pos
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	if !p.curTokenIs(TokenString) {
		return nil, fmt.Errorf("%s: expected PNG file path string literal in .incpal directive", p.curTok.Pos)
	}
	filename := p.curTok.Literal
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	var count Expr
	if p.curTokenIs(TokenComma) {
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		var err error
		count, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	return &IncpalDirective{Filename: filename, Count: count, pos: pos}, nil
}

func (p *Parser) parseInstruction() (Statement, error) {
	pos := p.curTok.Pos
	mnemonic := strings.ToUpper(p.curTok.Literal)
	if err := p.nextToken(); err != nil {
		return nil, err
	}

	// 1. Implied / Accumulator (no operand)
	if p.curTokenIs(TokenEOL) || p.curTokenIs(TokenEOF) {
		if isAccOnly(mnemonic) {
			return &InstructionStmt{Mnemonic: mnemonic, Mode: cpu6502.ModeAccumulator, pos: pos}, nil
		}
		return &InstructionStmt{Mnemonic: mnemonic, Mode: cpu6502.ModeImplied, pos: pos}, nil
	}

	// 2. Explicit 'A' operand (Accumulator)
	if p.curTokenIs(TokenRegA) && (p.peekTokenIs(TokenEOL) || p.peekTokenIs(TokenEOF)) {
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		return &InstructionStmt{Mnemonic: mnemonic, Mode: cpu6502.ModeAccumulator, pos: pos}, nil
	}

	// 3. Immediate: #expr
	if p.curTokenIs(TokenHash) {
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		return &InstructionStmt{Mnemonic: mnemonic, Mode: cpu6502.ModeImmediate, Operand: expr, pos: pos}, nil
	}

	// 4. Indirect forms starting with '(':
	//    - (expr, X) -> IndirectX
	//    - (expr), Y -> IndirectY
	//    - (expr)    -> Indirect (JMP)
	if p.curTokenIs(TokenLParen) {
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		if p.curTokenIs(TokenComma) {
			// (expr, X)
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			if !p.curTokenIs(TokenRegX) {
				return nil, fmt.Errorf("%s: expected 'X' in indirect indexed addressing mode", p.curTok.Pos)
			}
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			if !p.curTokenIs(TokenRParen) {
				return nil, fmt.Errorf("%s: expected ')' after X", p.curTok.Pos)
			}
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			return &InstructionStmt{Mnemonic: mnemonic, Mode: cpu6502.ModeIndirectX, Operand: expr, pos: pos}, nil
		}

		if !p.curTokenIs(TokenRParen) {
			return nil, fmt.Errorf("%s: expected ')' in indirect addressing", p.curTok.Pos)
		}
		if err := p.nextToken(); err != nil {
			return nil, err
		}

		if p.curTokenIs(TokenComma) {
			// (expr), Y
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			if !p.curTokenIs(TokenRegY) {
				return nil, fmt.Errorf("%s: expected 'Y' after (expr),", p.curTok.Pos)
			}
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			return &InstructionStmt{Mnemonic: mnemonic, Mode: cpu6502.ModeIndirectY, Operand: expr, pos: pos}, nil
		}

		// (expr) -> JMP indirect
		return &InstructionStmt{Mnemonic: mnemonic, Mode: cpu6502.ModeIndirect, Operand: expr, pos: pos}, nil
	}

	// 5. Memory expressions:
	//    expr, X
	//    expr, Y
	//    expr (Relative for branch instructions, or ZeroPage / Absolute)
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.curTokenIs(TokenComma) {
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		if p.curTokenIs(TokenRegX) {
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			mode := cpu6502.ModeAbsoluteX
			// Check if expression is explicitly forced to zero-page via z:
			if u, ok := expr.(*UnaryExpr); ok && u.Op == UnaryZeroPage {
				mode = cpu6502.ModeZeroPageX
			}
			return &InstructionStmt{Mnemonic: mnemonic, Mode: mode, Operand: expr, pos: pos}, nil
		} else if p.curTokenIs(TokenRegY) {
			if err := p.nextToken(); err != nil {
				return nil, err
			}
			mode := cpu6502.ModeAbsoluteY
			if u, ok := expr.(*UnaryExpr); ok && u.Op == UnaryZeroPage {
				mode = cpu6502.ModeZeroPageY
			}
			return &InstructionStmt{Mnemonic: mnemonic, Mode: mode, Operand: expr, pos: pos}, nil
		}
		return nil, fmt.Errorf("%s: expected 'X' or 'Y' after comma", p.curTok.Pos)
	}

	if cpu6502.IsBranch(mnemonic) {
		return &InstructionStmt{Mnemonic: mnemonic, Mode: cpu6502.ModeRelative, Operand: expr, pos: pos}, nil
	}

	mode := cpu6502.ModeAbsolute
	if u, ok := expr.(*UnaryExpr); ok && u.Op == UnaryZeroPage {
		mode = cpu6502.ModeZeroPage
	}
	return &InstructionStmt{Mnemonic: mnemonic, Mode: mode, Operand: expr, pos: pos}, nil
}

func isAccOnly(mnemonic string) bool {
	switch mnemonic {
	case "ASL", "LSR", "ROL", "ROR":
		return true
	default:
		return false
	}
}

// Expression parsing with precedence
func (p *Parser) parseExpression() (Expr, error) {
	return p.parseLogicalOr()
}

func (p *Parser) parseLogicalOr() (Expr, error) {
	left, err := p.parseLogicalAnd()
	if err != nil {
		return nil, err
	}
	for p.curTokenIs(TokenLogicalOr) {
		pos := p.curTok.Pos
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseLogicalAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: BinLogOr, Left: left, Right: right, pos: pos}
	}
	return left, nil
}

func (p *Parser) parseLogicalAnd() (Expr, error) {
	left, err := p.parseBitOr()
	if err != nil {
		return nil, err
	}
	for p.curTokenIs(TokenLogicalAnd) {
		pos := p.curTok.Pos
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseBitOr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: BinLogAnd, Left: left, Right: right, pos: pos}
	}
	return left, nil
}

func (p *Parser) parseBitOr() (Expr, error) {
	left, err := p.parseBitXor()
	if err != nil {
		return nil, err
	}
	for p.curTokenIs(TokenPipe) {
		pos := p.curTok.Pos
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseBitXor()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: BinBitOr, Left: left, Right: right, pos: pos}
	}
	return left, nil
}

func (p *Parser) parseBitXor() (Expr, error) {
	left, err := p.parseBitAnd()
	if err != nil {
		return nil, err
	}
	for p.curTokenIs(TokenCaret) {
		pos := p.curTok.Pos
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseBitAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: BinBitXor, Left: left, Right: right, pos: pos}
	}
	return left, nil
}

func (p *Parser) parseBitAnd() (Expr, error) {
	left, err := p.parseEquality()
	if err != nil {
		return nil, err
	}
	for p.curTokenIs(TokenAmp) {
		pos := p.curTok.Pos
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: BinBitAnd, Left: left, Right: right, pos: pos}
	}
	return left, nil
}

func (p *Parser) parseEquality() (Expr, error) {
	left, err := p.parseRelational()
	if err != nil {
		return nil, err
	}
	for p.curTokenIs(TokenEqEq) || p.curTokenIs(TokenBangEq) || p.curTokenIs(TokenNotEqCa65) || (p.curTokenIs(TokenAssign) && !p.peekTokenIs(TokenEOL)) {
		var op BinaryOp
		switch p.curTok.Type {
		case TokenEqEq, TokenAssign:
			op = BinEq
		case TokenBangEq, TokenNotEqCa65:
			op = BinNotEq
		}
		pos := p.curTok.Pos
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseRelational()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right, pos: pos}
	}
	return left, nil
}

func (p *Parser) parseRelational() (Expr, error) {
	left, err := p.parseShift()
	if err != nil {
		return nil, err
	}
	for p.curTokenIs(TokenLt) || p.curTokenIs(TokenLtEq) || p.curTokenIs(TokenGt) || p.curTokenIs(TokenGtEq) {
		var op BinaryOp
		switch p.curTok.Type {
		case TokenLt:
			op = BinLt
		case TokenLtEq:
			op = BinLtEq
		case TokenGt:
			op = BinGt
		case TokenGtEq:
			op = BinGtEq
		}
		pos := p.curTok.Pos
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseShift()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right, pos: pos}
	}
	return left, nil
}

func (p *Parser) parseShift() (Expr, error) {
	left, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	for p.curTokenIs(TokenLShift) || p.curTokenIs(TokenRShift) {
		op := BinShl
		if p.curTokenIs(TokenRShift) {
			op = BinShr
		}
		pos := p.curTok.Pos
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseAdd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right, pos: pos}
	}
	return left, nil
}

func (p *Parser) parseAdd() (Expr, error) {
	left, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for p.curTokenIs(TokenPlus) || p.curTokenIs(TokenMinus) {
		op := BinAdd
		if p.curTokenIs(TokenMinus) {
			op = BinSub
		}
		pos := p.curTok.Pos
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right, pos: pos}
	}
	return left, nil
}

func (p *Parser) parseMul() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.curTokenIs(TokenStar) || p.curTokenIs(TokenSlash) || p.curTokenIs(TokenPercent) {
		var op BinaryOp
		switch p.curTok.Type {
		case TokenStar:
			op = BinMul
		case TokenSlash:
			op = BinDiv
		case TokenPercent:
			op = BinMod
		}
		pos := p.curTok.Pos
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right, pos: pos}
	}
	return left, nil
}

func (p *Parser) parseUnary() (Expr, error) {
	pos := p.curTok.Pos
	switch p.curTok.Type {
	case TokenPlus:
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		return &UnaryExpr{Op: UnaryPos, Right: right, pos: pos}, err
	case TokenMinus:
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		return &UnaryExpr{Op: UnaryNeg, Right: right, pos: pos}, err
	case TokenTilde:
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		return &UnaryExpr{Op: UnaryBitNot, Right: right, pos: pos}, err
	case TokenBang:
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		return &UnaryExpr{Op: UnaryLogNot, Right: right, pos: pos}, err
	case TokenLt:
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		return &UnaryExpr{Op: UnaryLowByte, Right: right, pos: pos}, err
	case TokenGt:
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		return &UnaryExpr{Op: UnaryHighByte, Right: right, pos: pos}, err
	case TokenCaret:
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		return &UnaryExpr{Op: UnaryBankByte, Right: right, pos: pos}, err
	case TokenZPrefix:
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		return &UnaryExpr{Op: UnaryZeroPage, Right: right, pos: pos}, err
	case TokenAPrefix:
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		return &UnaryExpr{Op: UnaryAbsolute, Right: right, pos: pos}, err
	default:
		return p.parsePrimary()
	}
}

func (p *Parser) parsePrimary() (Expr, error) {
	pos := p.curTok.Pos
	switch p.curTok.Type {
	case TokenNumber, TokenChar:
		val := p.curTok.NumValue
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		return &NumberExpr{Value: val, pos: pos}, nil
	case TokenIdent:
		name := p.curTok.Literal
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		return &SymbolExpr{Name: name, pos: pos}, nil
	case TokenLocalIdent:
		name := p.curTok.Literal
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		return &SymbolExpr{Name: name, pos: pos}, nil
	case TokenAnonRef:
		name := p.curTok.Literal
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		return &SymbolExpr{Name: name, pos: pos}, nil
	case TokenLParen:
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if !p.curTokenIs(TokenRParen) {
			return nil, fmt.Errorf("%s: expected ')'", p.curTok.Pos)
		}
		if err := p.nextToken(); err != nil {
			return nil, err
		}
		return expr, nil
	default:
		return nil, fmt.Errorf("%s: unexpected token %v (%q) in expression", pos, p.curTok.Type, p.curTok.Literal)
	}
}
