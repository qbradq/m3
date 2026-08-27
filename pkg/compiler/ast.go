package compiler

// Node represents an AST node.
type Node interface {
	Pos() Position
}

// Expr represents an expression node in the AST.
type Expr interface {
	Node
	exprNode()
}

// Stmt represents a statement node in the AST.
type Stmt interface {
	Node
	stmtNode()
}

// Decl represents a declaration node in the AST.
type Decl interface {
	Node
	declNode()
}

// TypeSpec represents a type specification in the AST.
type TypeSpec interface {
	Node
	typeSpecNode()
	String() string
}

// StorageClass specifies the memory segment where a variable is allocated.
type StorageClass int

const (
	StorageRAM StorageClass = iota // Default RAM ($0300-$07FF)
	StorageZP                      // Zero Page ($0000-$00FF)
	StorageWRAM                    // Work RAM ($6000-$7FFF)
)

func (s StorageClass) String() string {
	switch s {
	case StorageZP:
		return "zp"
	case StorageWRAM:
		return "wram"
	default:
		return "ram"
	}
}

// BankSpec defines PRG-ROM bank placement.
type BankSpec struct {
	IsAuto bool
	Value  Expr
	pos    Position
}

func (b *BankSpec) Pos() Position { return b.pos }

// ----------------------------------------------------------------------------
// Type Specifications
// ----------------------------------------------------------------------------

// NamedType represents a basic type (uint8, uint16, etc.) or user-defined type name (Actor).
type NamedType struct {
	Name string
	pos  Position
}

func (t *NamedType) Pos() Position     { return t.pos }
func (t *NamedType) typeSpecNode()     {}
func (t *NamedType) String() string    { return t.Name }

// PointerType represents *T.
type PointerType struct {
	Elem TypeSpec
	pos  Position
}

func (t *PointerType) Pos() Position     { return t.pos }
func (t *PointerType) typeSpecNode()     {}
func (t *PointerType) String() string    { return "*" + t.Elem.String() }

// ArrayType represents T[length] or T[] or [length]T.
type ArrayType struct {
	Elem   TypeSpec
	Length Expr // May be nil for inferred length
	pos    Position
}

func (t *ArrayType) Pos() Position     { return t.pos }
func (t *ArrayType) typeSpecNode()     {}
func (t *ArrayType) String() string {
	if t.Length != nil {
		return t.Elem.String() + "[...]"
	}
	return t.Elem.String() + "[]"
}

// StructField represents a single field in a struct.
type StructField struct {
	Name string
	Type TypeSpec
	pos  Position
}

func (f *StructField) Pos() Position { return f.pos }

// StructType represents struct { field1 type1; ... }.
type StructType struct {
	Fields []*StructField
	pos    Position
}

func (t *StructType) Pos() Position     { return t.pos }
func (t *StructType) typeSpecNode()     {}
func (t *StructType) String() string    { return "struct" }

// ----------------------------------------------------------------------------
// Top-Level Declarations and File
// ----------------------------------------------------------------------------

// SourceFile represents an entire .m3 compilation unit.
type SourceFile struct {
	PackageName string
	Imports     []*ImportDecl
	Decls       []Decl
	pos         Position
}

func (f *SourceFile) Pos() Position { return f.pos }

// ImportDecl represents an import statement.
type ImportDecl struct {
	Path string
	pos  Position
}

func (i *ImportDecl) Pos() Position { return i.pos }
func (i *ImportDecl) declNode()     {}

// VarDecl represents a variable declaration: var name type[len] storage [= init]
type VarDecl struct {
	Name    string
	Type    TypeSpec
	Length  Expr // Optional array length
	Storage StorageClass
	Init    Expr // Optional initializer
	pos     Position
}

func (v *VarDecl) Pos() Position { return v.pos }
func (v *VarDecl) declNode()     {}
func (v *VarDecl) stmtNode()     {}

// ConstDecl represents a constant or ROM data definition:
// const identifier type[length] bank = value
type ConstDecl struct {
	Name   string
	Type   TypeSpec
	Length Expr // Optional array length
	Bank   *BankSpec
	Value  Expr
	pos    Position
}

func (c *ConstDecl) Pos() Position { return c.pos }
func (c *ConstDecl) declNode()     {}
func (c *ConstDecl) stmtNode()     {}

// DefineDecl represents a compile-time constant definition: define identifier const_expr
type DefineDecl struct {
	Name  string
	Value Expr
	pos   Position
}

func (d *DefineDecl) Pos() Position { return d.pos }
func (d *DefineDecl) declNode()     {}
func (d *DefineDecl) stmtNode()     {}

// TypeDecl represents a type definition: type Actor struct { ... }
type TypeDecl struct {
	Name string
	Type TypeSpec
	pos  Position
}

func (t *TypeDecl) Pos() Position { return t.pos }
func (t *TypeDecl) declNode()     {}

// Param represents a function parameter.
type Param struct {
	Name string
	Type TypeSpec
	pos  Position
}

func (p *Param) Pos() Position { return p.pos }

// FuncDecl represents a function definition.
type FuncDecl struct {
	Name       string
	Params     []*Param
	ReturnType TypeSpec // nil if void
	Bank       *BankSpec
	Pragmas    []string // e.g. ["export nmi"]
	Body       *BlockStmt
	pos        Position
}

func (f *FuncDecl) Pos() Position { return f.pos }
func (f *FuncDecl) declNode()     {}

// ----------------------------------------------------------------------------
// Statements
// ----------------------------------------------------------------------------

// BlockStmt represents a sequence of statements enclosed in braces.
type BlockStmt struct {
	Stmts []Stmt
	pos   Position
}

func (b *BlockStmt) Pos() Position { return b.pos }
func (b *BlockStmt) stmtNode()     {}

// VarDeclStmt wraps a VarDecl as a statement inside functions.
type VarDeclStmt struct {
	Decl *VarDecl
	pos  Position
}

func (v *VarDeclStmt) Pos() Position { return v.pos }
func (v *VarDeclStmt) stmtNode()     {}

// ConstDeclStmt wraps a ConstDecl as a statement inside functions.
type ConstDeclStmt struct {
	Decl *ConstDecl
	pos  Position
}

func (c *ConstDeclStmt) Pos() Position { return c.pos }
func (c *ConstDeclStmt) stmtNode()     {}

// DefineDeclStmt wraps a DefineDecl as a statement inside functions.
type DefineDeclStmt struct {
	Decl *DefineDecl
	pos  Position
}

func (d *DefineDeclStmt) Pos() Position { return d.pos }
func (d *DefineDeclStmt) stmtNode()     {}

// AssignStmt represents an assignment: Left op Right (=, +=, -=, etc.)
type AssignStmt struct {
	Left  Expr
	Op    TokenType
	Right Expr
	pos   Position
}

func (a *AssignStmt) Pos() Position { return a.pos }
func (a *AssignStmt) stmtNode()     {}

// IncDecStmt represents an increment or decrement statement: x++, x--
type IncDecStmt struct {
	Expr  Expr
	IsInc bool
	pos   Position
}

func (i *IncDecStmt) Pos() Position { return i.pos }
func (i *IncDecStmt) stmtNode()     {}

// ShortVarDeclStmt represents a short variable declaration: i := 0
type ShortVarDeclStmt struct {
	Name  string
	Value Expr
	pos   Position
}

func (s *ShortVarDeclStmt) Pos() Position { return s.pos }
func (s *ShortVarDeclStmt) stmtNode()     {}

// IfStmt represents an if statement: if [init;] cond { then } else { else }
type IfStmt struct {
	Init Stmt // Optional init statement (e.g. i := 0)
	Cond Expr
	Then *BlockStmt
	Else Stmt // Either *BlockStmt or *IfStmt (for else if)
	pos  Position
}

func (i *IfStmt) Pos() Position { return i.pos }
func (i *IfStmt) stmtNode()     {}

// ForStmt represents a for loop (counted, while-style, or infinite).
type ForStmt struct {
	Init Stmt // Optional (e.g. i := uint8(0))
	Cond Expr // Optional (nil for infinite loop)
	Post Stmt // Optional (e.g. i++)
	Body *BlockStmt
	pos  Position
}

func (f *ForStmt) Pos() Position { return f.pos }
func (f *ForStmt) stmtNode()     {}

// CaseClause represents a case/default branch in a switch statement.
type CaseClause struct {
	Values []Expr // Empty for default
	Body   []Stmt
	pos    Position
}

func (c *CaseClause) Pos() Position { return c.pos }

// SwitchStmt represents a switch statement.
type SwitchStmt struct {
	Expr  Expr
	Cases []*CaseClause
	pos   Position
}

func (s *SwitchStmt) Pos() Position { return s.pos }
func (s *SwitchStmt) stmtNode()     {}

// ReturnStmt represents a return statement.
type ReturnStmt struct {
	Value Expr // Optional
	pos   Position
}

func (r *ReturnStmt) Pos() Position { return r.pos }
func (r *ReturnStmt) stmtNode()     {}

// BreakStmt represents a break statement.
type BreakStmt struct {
	pos Position
}

func (b *BreakStmt) Pos() Position { return b.pos }
func (b *BreakStmt) stmtNode()     {}

// ContinueStmt represents a continue statement.
type ContinueStmt struct {
	pos Position
}

func (c *ContinueStmt) Pos() Position { return c.pos }
func (c *ContinueStmt) stmtNode()     {}

// ExprStmt represents an expression evaluated as a statement (e.g., function call).
type ExprStmt struct {
	Expr Expr
	pos  Position
}

func (e *ExprStmt) Pos() Position { return e.pos }
func (e *ExprStmt) stmtNode()     {}

// AsmStmt represents an inline assembly block: asm { ... }
type AsmStmt struct {
	Body string
	pos  Position
}

func (a *AsmStmt) Pos() Position { return a.pos }
func (a *AsmStmt) stmtNode()     {}

// ----------------------------------------------------------------------------
// Expressions
// ----------------------------------------------------------------------------

// Ident represents an identifier expression.
type Ident struct {
	Name string
	pos  Position
}

func (i *Ident) Pos() Position { return i.pos }
func (i *Ident) exprNode()     {}

// NumberLit represents an integer number literal.
type NumberLit struct {
	Value int64
	Raw   string
	pos   Position
}

func (n *NumberLit) Pos() Position { return n.pos }
func (n *NumberLit) exprNode()     {}

// StringLit represents a string literal.
type StringLit struct {
	Value string
	pos   Position
}

func (s *StringLit) Pos() Position { return s.pos }
func (s *StringLit) exprNode()     {}

// CharLit represents a character literal.
type CharLit struct {
	Value rune
	pos   Position
}

func (c *CharLit) Pos() Position { return c.pos }
func (c *CharLit) exprNode()     {}

// BoolLit represents a boolean literal (true or false).
type BoolLit struct {
	Value bool
	pos   Position
}

func (b *BoolLit) Pos() Position { return b.pos }
func (b *BoolLit) exprNode()     {}

// ArrayLit represents an array literal like [8]uint8{16, 48, 80}.
type ArrayLit struct {
	Length   Expr
	ElemType TypeSpec
	Elements []Expr
	pos      Position
}

func (a *ArrayLit) Pos() Position { return a.pos }
func (a *ArrayLit) exprNode()     {}

// UnaryExpr represents a unary operation: +x, -x, !x, ^x, *x, &x, <x, >x, etc.
type UnaryExpr struct {
	Op      TokenType
	Operand Expr
	pos     Position
}

func (u *UnaryExpr) Pos() Position { return u.pos }
func (u *UnaryExpr) exprNode()     {}

// BinaryExpr represents a binary operation: a + b, a == b, a & b, etc.
type BinaryExpr struct {
	Left  Expr
	Op    TokenType
	Right Expr
	pos   Position
}

func (b *BinaryExpr) Pos() Position { return b.pos }
func (b *BinaryExpr) exprNode()     {}

// ParenExpr represents a parenthesized expression: (a + b)
type ParenExpr struct {
	Expr Expr
	pos  Position
}

func (p *ParenExpr) Pos() Position { return p.pos }
func (p *ParenExpr) exprNode()     {}

// CallExpr represents a function call or type conversion: foo(a, b) or uint8(0)
type CallExpr struct {
	Func Expr
	Args []Expr
	pos  Position
}

func (c *CallExpr) Pos() Position { return c.pos }
func (c *CallExpr) exprNode()     {}

// IndexExpr represents an index expression: actors[i]
type IndexExpr struct {
	Array Expr
	Index Expr
	pos   Position
}

func (i *IndexExpr) Pos() Position { return i.pos }
func (i *IndexExpr) exprNode()     {}

// MemberExpr represents a member access: actors[i].x or obj.field
type MemberExpr struct {
	Target Expr
	Member string
	pos    Position
}

func (m *MemberExpr) Pos() Position { return m.pos }
func (m *MemberExpr) exprNode()     {}

// IncbinExpr represents incbin("path")
type IncbinExpr struct {
	Path string
	pos  Position
}

func (i *IncbinExpr) Pos() Position { return i.pos }
func (i *IncbinExpr) exprNode()     {}

// IncchrExpr represents incchr("path")
type IncchrExpr struct {
	Path string
	pos  Position
}

func (i *IncchrExpr) Pos() Position { return i.pos }
func (i *IncchrExpr) exprNode()     {}

// IncpalExpr represents incpal("path" [, count])
type IncpalExpr struct {
	Path  string
	Count Expr // Optional (nil if omitted, defaults to 4)
	pos   Position
}

func (i *IncpalExpr) Pos() Position { return i.pos }
func (i *IncpalExpr) exprNode()     {}

