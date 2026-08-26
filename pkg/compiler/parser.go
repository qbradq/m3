package compiler

import (
	"fmt"
	"strings"
)

// Parser parses a stream of tokens into an AST.
type Parser struct {
	lexer     *Lexer
	curToken  Token
	peekToken Token
	errors    []string
}

// NewParser creates a new Parser for the lexer.
func NewParser(lexer *Lexer) *Parser {
	p := &Parser{lexer: lexer}
	// Read two tokens so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.lexer.NextToken()
}

func (p *Parser) curTokenIs(t TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expect(t TokenType) bool {
	if p.curTokenIs(t) {
		p.nextToken()
		return true
	}
	p.errorf("expected %s, got %s (%q)", t, p.curToken.Type, p.curToken.Literal)
	return false
}

func (p *Parser) errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf("%s: %s", p.curToken.Pos, fmt.Sprintf(format, args...))
	p.errors = append(p.errors, msg)
}

// Errors returns all parser error messages.
func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) skipSemicolons() {
	for p.curTokenIs(TokenSemicolon) {
		p.nextToken()
	}
}

// ParseSourceFile parses an entire .m3 source file.
func (p *Parser) ParseSourceFile() (*SourceFile, error) {
	file := &SourceFile{
		pos: p.curToken.Pos,
	}

	p.skipSemicolons()

	// Optional package statement
	if p.curTokenIs(TokenPackage) {
		p.nextToken()
		if !p.curTokenIs(TokenIdent) {
			p.errorf("expected package name identifier, got %s", p.curToken.Type)
			return nil, p.errorResult()
		}
		file.PackageName = p.curToken.Literal
		p.nextToken()
		p.skipSemicolons()
	}

	// Imports & Declarations
	for !p.curTokenIs(TokenEOF) {
		p.skipSemicolons()
		if p.curTokenIs(TokenEOF) {
			break
		}

		switch p.curToken.Type {
		case TokenImport:
			imports, err := p.parseImportDecl()
			if err != nil {
				return nil, err
			}
			file.Imports = append(file.Imports, imports...)

		case TokenVar:
			decls, err := p.parseVarDecl()
			if err != nil {
				return nil, err
			}
			for _, d := range decls {
				file.Decls = append(file.Decls, d)
			}

		case TokenConst:
			decls, err := p.parseConstDecl()
			if err != nil {
				return nil, err
			}
			for _, d := range decls {
				file.Decls = append(file.Decls, d)
			}

		case TokenDefine:
			decls, err := p.parseDefineDecl()
			if err != nil {
				return nil, err
			}
			for _, d := range decls {
				file.Decls = append(file.Decls, d)
			}

		case TokenTypeKw:
			decl, err := p.parseTypeDecl()
			if err != nil {
				return nil, err
			}
			file.Decls = append(file.Decls, decl)

		case TokenFunc:
			decl, err := p.parseFuncDecl()
			if err != nil {
				return nil, err
			}
			file.Decls = append(file.Decls, decl)

		default:
			p.errorf("unexpected top-level token %s (%q)", p.curToken.Type, p.curToken.Literal)
			p.nextToken()
		}
	}

	if len(p.errors) > 0 {
		return nil, p.errorResult()
	}

	return file, nil
}

func (p *Parser) errorResult() error {
	return fmt.Errorf("parsing failed with %d error(s):\n%s", len(p.errors), strings.Join(p.errors, "\n"))
}

// ----------------------------------------------------------------------------
// Imports
// ----------------------------------------------------------------------------

func (p *Parser) parseImportDecl() ([]*ImportDecl, error) {
	pos := p.curToken.Pos
	p.nextToken() // consume 'import'

	var results []*ImportDecl

	if p.curTokenIs(TokenLParen) {
		// Grouped imports: import ( "a" "b" )
		p.nextToken() // consume '('
		p.skipSemicolons()
		for !p.curTokenIs(TokenRParen) && !p.curTokenIs(TokenEOF) {
			if !p.curTokenIs(TokenString) {
				p.errorf("expected import path string, got %s", p.curToken.Type)
				return nil, p.errorResult()
			}
			results = append(results, &ImportDecl{
				Path: p.curToken.Literal,
				pos:  p.curToken.Pos,
			})
			p.nextToken()
			p.skipSemicolons()
		}
		if !p.expect(TokenRParen) {
			return nil, p.errorResult()
		}
	} else {
		// Single import: import "path"
		if !p.curTokenIs(TokenString) {
			p.errorf("expected import path string, got %s", p.curToken.Type)
			return nil, p.errorResult()
		}
		results = append(results, &ImportDecl{
			Path: p.curToken.Literal,
			pos:  pos,
		})
		p.nextToken()
	}

	return results, nil
}

// ----------------------------------------------------------------------------
// Variable Declarations
// ----------------------------------------------------------------------------

func (p *Parser) parseVarDecl() ([]*VarDecl, error) {
	p.nextToken() // consume 'var'

	var decls []*VarDecl

	if p.curTokenIs(TokenLParen) {
		// Grouped var declaration: var ( ... )
		p.nextToken() // consume '('
		p.skipSemicolons()
		for !p.curTokenIs(TokenRParen) && !p.curTokenIs(TokenEOF) {
			d, err := p.parseSingleVarDecl()
			if err != nil {
				return nil, err
			}
			decls = append(decls, d)
			p.skipSemicolons()
		}
		if !p.expect(TokenRParen) {
			return nil, p.errorResult()
		}
	} else {
		d, err := p.parseSingleVarDecl()
		if err != nil {
			return nil, err
		}
		decls = append(decls, d)
	}

	return decls, nil
}

func (p *Parser) parseSingleVarDecl() (*VarDecl, error) {
	pos := p.curToken.Pos
	if !p.curTokenIs(TokenIdent) {
		p.errorf("expected variable name identifier, got %s (%q)", p.curToken.Type, p.curToken.Literal)
		return nil, p.errorResult()
	}
	name := p.curToken.Literal
	p.nextToken()

	// Type specification
	typeSpec, lengthExpr, err := p.parseTypeAndOptionalLength()
	if err != nil {
		return nil, err
	}

	// Storage specifier (optional: zp, zeropage, ram, wram, workram)
	storage := StorageRAM // default
	if p.curTokenIs(TokenZP) || p.curTokenIs(TokenZeroPage) {
		storage = StorageZP
		p.nextToken()
	} else if p.curTokenIs(TokenRAM) {
		storage = StorageRAM
		p.nextToken()
	} else if p.curTokenIs(TokenWRAM) || p.curTokenIs(TokenWorkRAM) {
		storage = StorageWRAM
		p.nextToken()
	}

	// Optional initializer: = <expr>
	var initExpr Expr
	if p.curTokenIs(TokenEq) {
		p.nextToken() // consume '='
		initExpr, err = p.parseExpression(0)
		if err != nil {
			return nil, err
		}
	}

	return &VarDecl{
		Name:    name,
		Type:    typeSpec,
		Length:  lengthExpr,
		Storage: storage,
		Init:    initExpr,
		pos:     pos,
	}, nil
}

// ----------------------------------------------------------------------------
// Constant Declarations
// ----------------------------------------------------------------------------

func (p *Parser) parseConstDecl() ([]*ConstDecl, error) {
	p.nextToken() // consume 'const'

	var decls []*ConstDecl

	if p.curTokenIs(TokenLParen) {
		// Grouped const declaration: const ( ... )
		p.nextToken() // consume '('
		p.skipSemicolons()
		for !p.curTokenIs(TokenRParen) && !p.curTokenIs(TokenEOF) {
			d, err := p.parseSingleConstDecl()
			if err != nil {
				return nil, err
			}
			decls = append(decls, d)
			p.skipSemicolons()
		}
		if !p.expect(TokenRParen) {
			return nil, p.errorResult()
		}
	} else {
		d, err := p.parseSingleConstDecl()
		if err != nil {
			return nil, err
		}
		decls = append(decls, d)
	}

	return decls, nil
}

func (p *Parser) parseSingleConstDecl() (*ConstDecl, error) {
	pos := p.curToken.Pos
	if !p.curTokenIs(TokenIdent) {
		p.errorf("expected constant name identifier, got %s (%q)", p.curToken.Type, p.curToken.Literal)
		return nil, p.errorResult()
	}
	name := p.curToken.Literal
	p.nextToken()

	// Type specification (optional or explicit, can have [length] or [])
	var typeSpec TypeSpec
	var lengthExpr Expr
	var err error

	if !p.curTokenIs(TokenBank) && !p.curTokenIs(TokenEq) {
		typeSpec, lengthExpr, err = p.parseTypeAndOptionalLength()
		if err != nil {
			return nil, err
		}
	}

	// Bank specifier (optional: bank <n> / bank auto)
	var bankSpec *BankSpec
	if p.curTokenIs(TokenBank) {
		bankSpec, err = p.parseBankSpec()
		if err != nil {
			return nil, err
		}
	}

	if !p.expect(TokenEq) {
		return nil, p.errorResult()
	}

	valueExpr, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	return &ConstDecl{
		Name:   name,
		Type:   typeSpec,
		Length: lengthExpr,
		Bank:   bankSpec,
		Value:  valueExpr,
		pos:    pos,
	}, nil
}

// ----------------------------------------------------------------------------
// Define Declarations
// ----------------------------------------------------------------------------

func (p *Parser) parseDefineDecl() ([]*DefineDecl, error) {
	p.nextToken() // consume 'define'

	var decls []*DefineDecl

	if p.curTokenIs(TokenLParen) {
		// Grouped define declaration: define ( ... )
		p.nextToken() // consume '('
		p.skipSemicolons()
		for !p.curTokenIs(TokenRParen) && !p.curTokenIs(TokenEOF) {
			d, err := p.parseSingleDefineDecl()
			if err != nil {
				return nil, err
			}
			decls = append(decls, d)
			p.skipSemicolons()
		}
		if !p.expect(TokenRParen) {
			return nil, p.errorResult()
		}
	} else {
		d, err := p.parseSingleDefineDecl()
		if err != nil {
			return nil, err
		}
		decls = append(decls, d)
	}

	return decls, nil
}

func (p *Parser) parseSingleDefineDecl() (*DefineDecl, error) {
	pos := p.curToken.Pos
	if !p.curTokenIs(TokenIdent) {
		p.errorf("expected identifier after define, got %s (%q)", p.curToken.Type, p.curToken.Literal)
		return nil, p.errorResult()
	}
	name := p.curToken.Literal
	p.nextToken()

	// Optional '='
	if p.curTokenIs(TokenEq) {
		p.nextToken()
	}

	valExpr, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	return &DefineDecl{
		Name:  name,
		Value: valExpr,
		pos:   pos,
	}, nil
}

func (p *Parser) parseBankSpec() (*BankSpec, error) {
	pos := p.curToken.Pos
	p.nextToken() // consume 'bank'

	if p.curTokenIs(TokenAuto) {
		p.nextToken()
		return &BankSpec{IsAuto: true, pos: pos}, nil
	}

	val, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	return &BankSpec{IsAuto: false, Value: val, pos: pos}, nil
}

// ----------------------------------------------------------------------------
// Type Declarations (Structs)
// ----------------------------------------------------------------------------

func (p *Parser) parseTypeDecl() (*TypeDecl, error) {
	pos := p.curToken.Pos
	p.nextToken() // consume 'type'

	if !p.curTokenIs(TokenIdent) {
		p.errorf("expected type name identifier, got %s (%q)", p.curToken.Type, p.curToken.Literal)
		return nil, p.errorResult()
	}
	name := p.curToken.Literal
	p.nextToken()

	if !p.curTokenIs(TokenStruct) {
		p.errorf("expected 'struct' in type declaration, got %s (%q)", p.curToken.Type, p.curToken.Literal)
		return nil, p.errorResult()
	}
	structPos := p.curToken.Pos
	p.nextToken() // consume 'struct'

	if !p.expect(TokenLBrace) {
		return nil, p.errorResult()
	}

	p.skipSemicolons()
	var fields []*StructField

	for !p.curTokenIs(TokenRBrace) && !p.curTokenIs(TokenEOF) {
		fieldPos := p.curToken.Pos
		if !p.curTokenIs(TokenIdent) {
			p.errorf("expected struct field identifier, got %s (%q)", p.curToken.Type, p.curToken.Literal)
			return nil, p.errorResult()
		}
		fieldName := p.curToken.Literal
		p.nextToken()

		fieldType, _, err := p.parseTypeAndOptionalLength()
		if err != nil {
			return nil, err
		}

		fields = append(fields, &StructField{
			Name: fieldName,
			Type: fieldType,
			pos:  fieldPos,
		})

		p.skipSemicolons()
	}

	if !p.expect(TokenRBrace) {
		return nil, p.errorResult()
	}

	return &TypeDecl{
		Name: name,
		Type: &StructType{Fields: fields, pos: structPos},
		pos:  pos,
	}, nil
}

// ----------------------------------------------------------------------------
// Function Declarations
// ----------------------------------------------------------------------------

func (p *Parser) parseFuncDecl() (*FuncDecl, error) {
	pos := p.curToken.Pos
	pragmas := p.lexer.TakePragmas()

	p.nextToken() // consume 'func'

	if !p.curTokenIs(TokenIdent) {
		p.errorf("expected function name identifier, got %s (%q)", p.curToken.Type, p.curToken.Literal)
		return nil, p.errorResult()
	}
	funcName := p.curToken.Literal
	p.nextToken()

	// Parameter list: (param1 type, param2 type, ...) or (src, dst *uint8[], len uint16)
	if !p.expect(TokenLParen) {
		return nil, p.errorResult()
	}

	var params []*Param
	for !p.curTokenIs(TokenRParen) && !p.curTokenIs(TokenEOF) {
		var groupNames []struct {
			name string
			pos  Position
		}

		for {
			if !p.curTokenIs(TokenIdent) {
				p.errorf("expected parameter name identifier, got %s (%q)", p.curToken.Type, p.curToken.Literal)
				return nil, p.errorResult()
			}
			paramPos := p.curToken.Pos
			paramName := p.curToken.Literal
			p.nextToken()

			groupNames = append(groupNames, struct {
				name string
				pos  Position
			}{name: paramName, pos: paramPos})

			if p.curTokenIs(TokenComma) {
				p.nextToken()
				continue
			}
			break
		}

		paramType, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}

		for _, g := range groupNames {
			params = append(params, &Param{
				Name: g.name,
				Type: paramType,
				pos:  g.pos,
			})
		}

		if p.curTokenIs(TokenComma) {
			p.nextToken()
		} else {
			break
		}
	}

	if !p.expect(TokenRParen) {
		return nil, p.errorResult()
	}

	// Optional return type
	var returnType TypeSpec
	if p.isTypeStart(p.curToken) {
		var err error
		returnType, err = p.parseTypeSpec()
		if err != nil {
			return nil, err
		}
	}

	// Optional bank specifier: bank <n> / bank auto
	var bankSpec *BankSpec
	if p.curTokenIs(TokenBank) {
		var err error
		bankSpec, err = p.parseBankSpec()
		if err != nil {
			return nil, err
		}
	}

	// Function body block: { ... }
	body, err := p.parseBlockStmt()
	if err != nil {
		return nil, err
	}

	return &FuncDecl{
		Name:       funcName,
		Params:     params,
		ReturnType: returnType,
		Bank:       bankSpec,
		Pragmas:    pragmas,
		Body:       body,
		pos:        pos,
	}, nil
}

// ----------------------------------------------------------------------------
// Type Parsing Helpers
// ----------------------------------------------------------------------------

func (p *Parser) isTypeStart(tok Token) bool {
	if tok.Type == TokenStar || tok.Type == TokenLBracket {
		return true
	}
	if tok.Type == TokenIdent {
		// Could be named type or struct name
		return true
	}
	return false
}

// parseTypeAndOptionalLength handles `type[length]` or `type[]` or `[length]type` or `*type` or `type`.
func (p *Parser) parseTypeAndOptionalLength() (TypeSpec, Expr, error) {
	ts, err := p.parseTypeSpec()
	if err != nil {
		return nil, nil, err
	}
	var lengthExpr Expr
	if arr, ok := ts.(*ArrayType); ok {
		lengthExpr = arr.Length
	}
	return ts, lengthExpr, nil
}

func (p *Parser) parseTypeSpec() (TypeSpec, error) {
	pos := p.curToken.Pos

	if p.curTokenIs(TokenStar) {
		p.nextToken() // consume '*'
		elem, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}
		return &PointerType{Elem: elem, pos: pos}, nil
	}

	if p.curTokenIs(TokenLBracket) {
		p.nextToken() // consume '['
		var lengthExpr Expr
		var err error
		if !p.curTokenIs(TokenRBracket) {
			lengthExpr, err = p.parseExpression(0)
			if err != nil {
				return nil, err
			}
		}
		if !p.expect(TokenRBracket) {
			return nil, p.errorResult()
		}
		elemType, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}
		return &ArrayType{Elem: elemType, Length: lengthExpr, pos: pos}, nil
	}

	if p.curTokenIs(TokenIdent) {
		name := p.curToken.Literal
		p.nextToken()
		var baseType TypeSpec = &NamedType{Name: name, pos: pos}
		if p.curTokenIs(TokenLBracket) {
			p.nextToken() // consume '['
			var lengthExpr Expr
			var err error
			if !p.curTokenIs(TokenRBracket) {
				lengthExpr, err = p.parseExpression(0)
				if err != nil {
					return nil, err
				}
			}
			if !p.expect(TokenRBracket) {
				return nil, p.errorResult()
			}
			baseType = &ArrayType{Elem: baseType, Length: lengthExpr, pos: pos}
		}
		return baseType, nil
	}

	p.errorf("expected type name or '*T', got %s (%q)", p.curToken.Type, p.curToken.Literal)
	return nil, p.errorResult()
}

// ----------------------------------------------------------------------------
// Statements
// ----------------------------------------------------------------------------

func (p *Parser) parseBlockStmt() (*BlockStmt, error) {
	pos := p.curToken.Pos
	if !p.expect(TokenLBrace) {
		return nil, p.errorResult()
	}

	p.skipSemicolons()
	var stmts []Stmt

	for !p.curTokenIs(TokenRBrace) && !p.curTokenIs(TokenEOF) {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		p.skipSemicolons()
	}

	if !p.expect(TokenRBrace) {
		return nil, p.errorResult()
	}

	return &BlockStmt{Stmts: stmts, pos: pos}, nil
}

func (p *Parser) parseStatement() (Stmt, error) {
	p.skipSemicolons()
	pos := p.curToken.Pos

	switch p.curToken.Type {
	case TokenVar:
		decls, err := p.parseVarDecl()
		if err != nil {
			return nil, err
		}
		if len(decls) == 1 {
			return &VarDeclStmt{Decl: decls[0], pos: pos}, nil
		}
		// If multiple in grouped var, return as block or first
		var stmts []Stmt
		for _, d := range decls {
			stmts = append(stmts, &VarDeclStmt{Decl: d, pos: d.Pos()})
		}
		return &BlockStmt{Stmts: stmts, pos: pos}, nil

	case TokenConst:
		decls, err := p.parseConstDecl()
		if err != nil {
			return nil, err
		}
		if len(decls) == 1 {
			return &ConstDeclStmt{Decl: decls[0], pos: pos}, nil
		}
		var stmts []Stmt
		for _, d := range decls {
			stmts = append(stmts, &ConstDeclStmt{Decl: d, pos: d.Pos()})
		}
		return &BlockStmt{Stmts: stmts, pos: pos}, nil

	case TokenDefine:
		decls, err := p.parseDefineDecl()
		if err != nil {
			return nil, err
		}
		if len(decls) == 1 {
			return &DefineDeclStmt{Decl: decls[0], pos: pos}, nil
		}
		var stmts []Stmt
		for _, d := range decls {
			stmts = append(stmts, &DefineDeclStmt{Decl: d, pos: d.Pos()})
		}
		return &BlockStmt{Stmts: stmts, pos: pos}, nil

	case TokenIf:
		return p.parseIfStmt()

	case TokenFor:
		return p.parseForStmt()

	case TokenSwitch:
		return p.parseSwitchStmt()

	case TokenReturn:
		p.nextToken() // consume 'return'
		var retVal Expr
		if !p.curTokenIs(TokenSemicolon) && !p.curTokenIs(TokenRBrace) {
			var err error
			retVal, err = p.parseExpression(0)
			if err != nil {
				return nil, err
			}
		}
		return &ReturnStmt{Value: retVal, pos: pos}, nil

	case TokenBreak:
		p.nextToken()
		return &BreakStmt{pos: pos}, nil

	case TokenContinue:
		p.nextToken()
		return &ContinueStmt{pos: pos}, nil

	case TokenAsm:
		body := p.curToken.Literal
		p.nextToken() // consume TokenAsm
		return &AsmStmt{Body: body, pos: pos}, nil

	case TokenLBrace:
		return p.parseBlockStmt()

	default:
		return p.parseSimpleStmt()
	}
}

func (p *Parser) parseIfStmt() (*IfStmt, error) {
	pos := p.curToken.Pos
	p.nextToken() // consume 'if'

	// Might have init statement: if init; cond { ... }
	firstExprOrStmt, err := p.parseSimpleStmtOrExpr()
	if err != nil {
		return nil, err
	}

	var initStmt Stmt
	var condExpr Expr

	if p.curTokenIs(TokenSemicolon) {
		p.nextToken() // consume ';'
		initStmt = firstExprOrStmt.(Stmt)
		condExpr, err = p.parseExpression(0)
		if err != nil {
			return nil, err
		}
	} else {
		// No init statement, firstExprOrStmt is condExpr
		switch v := firstExprOrStmt.(type) {
		case Expr:
			condExpr = v
		case *ExprStmt:
			condExpr = v.Expr
		default:
			p.errorf("expected boolean condition expression in if statement")
			return nil, p.errorResult()
		}
	}

	thenBlock, err := p.parseBlockStmt()
	if err != nil {
		return nil, err
	}

	var elseStmt Stmt
	if p.curTokenIs(TokenElse) {
		p.nextToken() // consume 'else'
		if p.curTokenIs(TokenIf) {
			elseStmt, err = p.parseIfStmt()
			if err != nil {
				return nil, err
			}
		} else {
			elseStmt, err = p.parseBlockStmt()
			if err != nil {
				return nil, err
			}
		}
	}

	return &IfStmt{
		Init: initStmt,
		Cond: condExpr,
		Then: thenBlock,
		Else: elseStmt,
		pos:  pos,
	}, nil
}

func (p *Parser) parseForStmt() (*ForStmt, error) {
	pos := p.curToken.Pos
	p.nextToken() // consume 'for'

	// Check if infinite loop: for { ... }
	if p.curTokenIs(TokenLBrace) {
		body, err := p.parseBlockStmt()
		if err != nil {
			return nil, err
		}
		return &ForStmt{Body: body, pos: pos}, nil
	}

	// Could be while-style `for cond { ... }` or counted `for init; cond; post { ... }`
	first, err := p.parseSimpleStmtOrExpr()
	if err != nil {
		return nil, err
	}

	if p.curTokenIs(TokenLBrace) {
		// While-style loop: for cond { ... }
		var cond Expr
		switch v := first.(type) {
		case Expr:
			cond = v
		case *ExprStmt:
			cond = v.Expr
		default:
			p.errorf("expected boolean condition for 'for' loop")
			return nil, p.errorResult()
		}
		body, err := p.parseBlockStmt()
		if err != nil {
			return nil, err
		}
		return &ForStmt{Cond: cond, Body: body, pos: pos}, nil
	}

	// Counted loop: for init; cond; post { ... }
	if !p.expect(TokenSemicolon) {
		return nil, p.errorResult()
	}

	initStmt, ok := first.(Stmt)
	if !ok {
		if ex, isEx := first.(Expr); isEx {
			initStmt = &ExprStmt{Expr: ex, pos: ex.Pos()}
		}
	}

	var condExpr Expr
	if !p.curTokenIs(TokenSemicolon) {
		condExpr, err = p.parseExpression(0)
		if err != nil {
			return nil, err
		}
	}
	if !p.expect(TokenSemicolon) {
		return nil, p.errorResult()
	}

	var postStmt Stmt
	if !p.curTokenIs(TokenLBrace) {
		postStmt, err = p.parseSimpleStmt()
		if err != nil {
			return nil, err
		}
	}

	body, err := p.parseBlockStmt()
	if err != nil {
		return nil, err
	}

	return &ForStmt{
		Init: initStmt,
		Cond: condExpr,
		Post: postStmt,
		Body: body,
		pos:  pos,
	}, nil
}

func (p *Parser) parseSwitchStmt() (*SwitchStmt, error) {
	pos := p.curToken.Pos
	p.nextToken() // consume 'switch'

	var expr Expr
	var err error
	if !p.curTokenIs(TokenLBrace) {
		expr, err = p.parseExpression(0)
		if err != nil {
			return nil, err
		}
	}

	if !p.expect(TokenLBrace) {
		return nil, p.errorResult()
	}

	p.skipSemicolons()
	var cases []*CaseClause

	for !p.curTokenIs(TokenRBrace) && !p.curTokenIs(TokenEOF) {
		casePos := p.curToken.Pos
		if p.curTokenIs(TokenCase) {
			p.nextToken() // consume 'case'
			var values []Expr
			for {
				v, err := p.parseExpression(0)
				if err != nil {
					return nil, err
				}
				values = append(values, v)
				if p.curTokenIs(TokenComma) {
					p.nextToken()
				} else {
					break
				}
			}
			if !p.expect(TokenColon) {
				return nil, p.errorResult()
			}
			p.skipSemicolons()
			var body []Stmt
			for !p.curTokenIs(TokenCase) && !p.curTokenIs(TokenDefault) && !p.curTokenIs(TokenRBrace) && !p.curTokenIs(TokenEOF) {
				s, err := p.parseStatement()
				if err != nil {
					return nil, err
				}
				if s != nil {
					body = append(body, s)
				}
				p.skipSemicolons()
			}
			cases = append(cases, &CaseClause{Values: values, Body: body, pos: casePos})

		} else if p.curTokenIs(TokenDefault) {
			p.nextToken() // consume 'default'
			if !p.expect(TokenColon) {
				return nil, p.errorResult()
			}
			p.skipSemicolons()
			var body []Stmt
			for !p.curTokenIs(TokenCase) && !p.curTokenIs(TokenDefault) && !p.curTokenIs(TokenRBrace) && !p.curTokenIs(TokenEOF) {
				s, err := p.parseStatement()
				if err != nil {
					return nil, err
				}
				if s != nil {
					body = append(body, s)
				}
				p.skipSemicolons()
			}
			cases = append(cases, &CaseClause{Values: nil, Body: body, pos: casePos})

		} else {
			p.errorf("expected 'case' or 'default', got %s (%q)", p.curToken.Type, p.curToken.Literal)
			return nil, p.errorResult()
		}
	}

	if !p.expect(TokenRBrace) {
		return nil, p.errorResult()
	}

	return &SwitchStmt{Expr: expr, Cases: cases, pos: pos}, nil
}

// parseSimpleStmtOrExpr parses either a simple statement (assignment, inc/dec, short decl) or an expression.
func (p *Parser) parseSimpleStmtOrExpr() (Node, error) {
	expr, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	pos := expr.Pos()

	// Check for short declaration: ident := expr
	if p.curTokenIs(TokenColonEq) {
		p.nextToken() // consume ':='
		ident, ok := expr.(*Ident)
		if !ok {
			p.errorf("left side of ':=' must be an identifier")
			return nil, p.errorResult()
		}
		val, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		return &ShortVarDeclStmt{Name: ident.Name, Value: val, pos: pos}, nil
	}

	// Check for inc/dec: expr++, expr--
	if p.curTokenIs(TokenPlusPlus) {
		p.nextToken()
		return &IncDecStmt{Expr: expr, IsInc: true, pos: pos}, nil
	}
	if p.curTokenIs(TokenMinusMinus) {
		p.nextToken()
		return &IncDecStmt{Expr: expr, IsInc: false, pos: pos}, nil
	}

	// Check for assignment: expr = val, expr += val, etc.
	if p.isAssignmentOp(p.curToken.Type) {
		op := p.curToken.Type
		p.nextToken()
		right, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		return &AssignStmt{Left: expr, Op: op, Right: right, pos: pos}, nil
	}

	return expr, nil
}

func (p *Parser) parseSimpleStmt() (Stmt, error) {
	node, err := p.parseSimpleStmtOrExpr()
	if err != nil {
		return nil, err
	}
	if stmt, ok := node.(Stmt); ok {
		return stmt, nil
	}
	if expr, ok := node.(Expr); ok {
		return &ExprStmt{Expr: expr, pos: expr.Pos()}, nil
	}
	return nil, nil
}

func (p *Parser) isAssignmentOp(tok TokenType) bool {
	switch tok {
	case TokenEq, TokenPlusEq, TokenMinusEq, TokenStarEq, TokenSlashEq,
		TokenAmpEq, TokenPipeEq, TokenCaretEq, TokenAmpCaretEq,
		TokenLShiftEq, TokenRShiftEq:
		return true
	default:
		return false
	}
}

// ----------------------------------------------------------------------------
// Expressions & Precedence
// ----------------------------------------------------------------------------

const (
	precLowest = iota
	precLogicalOr   // ||
	precLogicalAnd  // &&
	precComparison  // ==, !=, <, <=, >, >=
	precAdditive    // +, -, |, ^
	precMultiplicative // *, /, %, <<, >>, &, &^
	precUnary       // +x, -x, !x, ^x, *x, &x, <x, >x
	precPostfix     // (), [], .
)

func (p *Parser) getPrecedence(tok TokenType) int {
	switch tok {
	case TokenLogicalOr:
		return precLogicalOr
	case TokenLogicalAnd:
		return precLogicalAnd
	case TokenEqEq, TokenBangEq, TokenLt, TokenLtEq, TokenGt, TokenGtEq:
		return precComparison
	case TokenPlus, TokenMinus, TokenPipe, TokenCaret:
		return precAdditive
	case TokenStar, TokenSlash, TokenPercent, TokenLShift, TokenRShift, TokenAmp, TokenAmpCaret:
		return precMultiplicative
	default:
		return precLowest
	}
}

func (p *Parser) parseExpression(precedence int) (Expr, error) {
	left, err := p.parsePrefixExpression()
	if err != nil {
		return nil, err
	}

	for !p.curTokenIs(TokenSemicolon) && !p.curTokenIs(TokenEOF) && precedence < p.getPrecedence(p.curToken.Type) {
		op := p.curToken.Type
		p.nextToken()

		right, err := p.parseExpression(p.getPrecedence(op))
		if err != nil {
			return nil, err
		}

		left = &BinaryExpr{
			Left:  left,
			Op:    op,
			Right: right,
			pos:   left.Pos(),
		}
	}

	return left, nil
}

func (p *Parser) parsePrefixExpression() (Expr, error) {
	pos := p.curToken.Pos

	// Unary operators: +, -, !, ^, ~, *, &, <, >
	switch p.curToken.Type {
	case TokenPlus, TokenMinus, TokenBang, TokenCaret, TokenTilde, TokenStar, TokenAmp, TokenLt, TokenGt:
		op := p.curToken.Type
		p.nextToken()
		operand, err := p.parseExpression(precUnary)
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: op, Operand: operand, pos: pos}, nil

	case TokenNumber:
		val := p.curToken.IntValue
		raw := p.curToken.Literal
		p.nextToken()
		return &NumberLit{Value: val, Raw: raw, pos: pos}, nil

	case TokenString:
		val := p.curToken.Literal
		p.nextToken()
		return &StringLit{Value: val, pos: pos}, nil

	case TokenChar:
		val := rune(p.curToken.IntValue)
		p.nextToken()
		return &CharLit{Value: val, pos: pos}, nil

	case TokenTrue:
		p.nextToken()
		return &BoolLit{Value: true, pos: pos}, nil

	case TokenFalse:
		p.nextToken()
		return &BoolLit{Value: false, pos: pos}, nil

	case TokenIdent, TokenBank:
		ident := &Ident{Name: p.curToken.Literal, pos: pos}
		p.nextToken()
		return p.parsePostfix(ident)

	case TokenLParen:
		p.nextToken() // consume '('
		expr, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		if !p.expect(TokenRParen) {
			return nil, p.errorResult()
		}
		return p.parsePostfix(&ParenExpr{Expr: expr, pos: pos})

	case TokenLBracket:
		// Array literal: [length]type{ ... }
		return p.parseArrayLiteral()

	default:
		p.errorf("unexpected token in expression: %s (%q)", p.curToken.Type, p.curToken.Literal)
		return nil, p.errorResult()
	}
}

func (p *Parser) parsePostfix(left Expr) (Expr, error) {
	for {
		if p.curTokenIs(TokenLParen) {
			// Call expression: foo(arg1, arg2)
			callPos := p.curToken.Pos
			p.nextToken() // consume '('
			var args []Expr
			for !p.curTokenIs(TokenRParen) && !p.curTokenIs(TokenEOF) {
				arg, err := p.parseExpression(0)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
				if p.curTokenIs(TokenComma) {
					p.nextToken()
				} else {
					break
				}
			}
			if !p.expect(TokenRParen) {
				return nil, p.errorResult()
			}
			left = &CallExpr{Func: left, Args: args, pos: callPos}

		} else if p.curTokenIs(TokenLBracket) {
			// Index expression: left[index]
			indexPos := p.curToken.Pos
			p.nextToken() // consume '['
			index, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			if !p.expect(TokenRBracket) {
				return nil, p.errorResult()
			}
			left = &IndexExpr{Array: left, Index: index, pos: indexPos}

		} else if p.curTokenIs(TokenDot) {
			// Member access: left.member
			dotPos := p.curToken.Pos
			p.nextToken() // consume '.'
			if !p.curTokenIs(TokenIdent) {
				p.errorf("expected member identifier after '.', got %s", p.curToken.Type)
				return nil, p.errorResult()
			}
			memberName := p.curToken.Literal
			p.nextToken()
			left = &MemberExpr{Target: left, Member: memberName, pos: dotPos}

		} else {
			break
		}
	}

	return left, nil
}

func (p *Parser) parseArrayLiteral() (*ArrayLit, error) {
	pos := p.curToken.Pos
	p.nextToken() // consume '['

	var lengthExpr Expr
	var err error
	if !p.curTokenIs(TokenRBracket) {
		lengthExpr, err = p.parseExpression(0)
		if err != nil {
			return nil, err
		}
	}
	if !p.expect(TokenRBracket) {
		return nil, p.errorResult()
	}

	elemType, err := p.parseTypeSpec()
	if err != nil {
		return nil, err
	}

	if !p.expect(TokenLBrace) {
		return nil, p.errorResult()
	}

	p.skipSemicolons()
	var elements []Expr

	for !p.curTokenIs(TokenRBrace) && !p.curTokenIs(TokenEOF) {
		elem, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		elements = append(elements, elem)
		p.skipSemicolons()
		if p.curTokenIs(TokenComma) {
			p.nextToken()
			p.skipSemicolons()
		} else {
			break
		}
	}

	if !p.expect(TokenRBrace) {
		return nil, p.errorResult()
	}

	return &ArrayLit{
		Length:   lengthExpr,
		ElemType: elemType,
		Elements: elements,
		pos:      pos,
	}, nil
}
