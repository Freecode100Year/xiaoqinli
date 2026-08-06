package ast

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// MaxASTBytes is the maximum allowed size for an AST binary payload.
	MaxASTBytes = 2 << 20 // 2 MB

	// MaxStringLen is the maximum allowed length for a single string in the binary format.
	MaxStringLen = 1 << 20 // 1 MB

	// MaxChildCount is the maximum number of children a node list can contain.
	MaxChildCount = 65536

	// MaxDecodeDepth 是 decodeNode 递归调用的最大深度，防止栈溢出攻击。
	MaxDecodeDepth = 256

	// MaxTypeDepth 是 readTypeExpr 递归调用的最大深度，防止栈溢出攻击。
	MaxTypeDepth = 64
)

// NodeHash is a SHA256 hash of a node's binary representation (for Merkle tree verification).
type NodeHash [32]byte

// NodeKind represents the type identifier for binary serialization.
type NodeKind byte

const (
	KindProgram NodeKind = iota + 1
	KindFunctionDecl
	KindReturnStmt
	KindVarDecl
	KindAssignStmt
	KindIfStmt
	KindWhileStmt
	KindForStmt
	KindBreakStmt
	KindContinueStmt
	KindExprStmt
	KindStructDecl
	KindEnumDecl
	KindMatchExpr
	KindBinaryExpr
	KindUnaryExpr
	KindCallExpr
	KindLiteral
	KindIdent
	KindMemberExpr
	KindStructLit
	KindArrayLit
	KindIndexExpr
	KindIfExpr
	KindNewExpr
	KindAwaitExpr
	KindLambda
	KindClassDecl
	KindSwitchStmt
	KindMapLiteral
	KindArrayLiteral
	KindImportDecl
)

// EncodeWithHash serializes an AST Node to a stable binary representation with a root hash.
func EncodeWithHash(node Node) ([]byte, NodeHash, error) {
	var buf bytes.Buffer
	if err := encodeNode(&buf, node); err != nil {
		return nil, NodeHash{}, err
	}
	data := buf.Bytes()
	hash := sha256.Sum256(data)
	return data, hash, nil
}

// Encode serializes an AST Node to a stable binary representation.
func Encode(node Node) ([]byte, error) {
	data, _, err := EncodeWithHash(node)
	return data, err
}

// Decode deserializes an AST Node from its stable binary representation.
func Decode(data []byte) (Node, error) {
	if len(data) > MaxASTBytes {
		return nil, fmt.Errorf("XQL_E413: payload too large %d > %d", len(data), MaxASTBytes)
	}
	buf := bytes.NewReader(data)
	// 入口调用，初始深度为 0
	return decodeNode(buf, 0)
}

// DecodeWithHash deserializes an AST Node and verifies its root hash.
func DecodeWithHash(data []byte, expectedHash NodeHash) (Node, error) {
	actualHash := sha256.Sum256(data)
	if actualHash != expectedHash {
		return nil, fmt.Errorf("XQL_E414: root hash mismatch: expected %x, got %x", expectedHash, actualHash)
	}
	return Decode(data)
}

func encodeNode(w io.Writer, n Node) error {
	if n == nil {
		return writeByte(w, 0)
	}

	switch node := n.(type) {
	case *Program:
		if err := writeByte(w, byte(KindProgram)); err != nil {
			return err
		}
		return writeNodeList(w, node.Decls)

	case *FunctionDecl:
		if err := writeByte(w, byte(KindFunctionDecl)); err != nil {
			return err
		}
		if err := writeString(w, node.Name); err != nil {
			return err
		}
		if len(node.Params) > MaxChildCount {
			return fmt.Errorf("XQL_E413: param count %d > %d", len(node.Params), MaxChildCount)
		}
		if err := binary.Write(w, binary.BigEndian, uint32(len(node.Params))); err != nil {
			return err
		}
		for _, p := range node.Params {
			if err := writeParam(w, p); err != nil {
				return err
			}
		}
		if err := writeTypeExpr(w, node.ReturnType); err != nil {
			return err
		}
		if err := writeStringList(w, node.Effects); err != nil {
			return err
		}
		if err := writeStringList(w, node.Grant); err != nil {
			return err
		}
		return writeNodeList(w, node.Body)

	case *ReturnStmt:
		if err := writeByte(w, byte(KindReturnStmt)); err != nil {
			return err
		}
		return encodeNode(w, node.Value)

	case *VarDecl:
		if err := writeByte(w, byte(KindVarDecl)); err != nil {
			return err
		}
		if err := writeString(w, node.Name); err != nil {
			return err
		}
		if err := writeTypeExpr(w, node.Type); err != nil {
			return err
		}
		return encodeNode(w, node.Value)

	case *AssignStmt:
		if err := writeByte(w, byte(KindAssignStmt)); err != nil {
			return err
		}
		if err := encodeNode(w, node.Target); err != nil {
			return err
		}
		return encodeNode(w, node.Value)

	case *IfStmt:
		if err := writeByte(w, byte(KindIfStmt)); err != nil {
			return err
		}
		if err := encodeNode(w, node.Cond); err != nil {
			return err
		}
		if err := writeNodeList(w, node.Then); err != nil {
			return err
		}
		return writeNodeList(w, node.Else)

	case *WhileStmt:
		if err := writeByte(w, byte(KindWhileStmt)); err != nil {
			return err
		}
		if err := encodeNode(w, node.Cond); err != nil {
			return err
		}
		return writeNodeList(w, node.Body)

	case *ForStmt:
		if err := writeByte(w, byte(KindForStmt)); err != nil {
			return err
		}
		if err := writeString(w, node.Form); err != nil {
			return err
		}
		if err := writeString(w, node.Var); err != nil {
			return err
		}
		if err := encodeNode(w, node.Start); err != nil {
			return err
		}
		if err := encodeNode(w, node.End); err != nil {
			return err
		}
		if err := encodeNode(w, node.Iterable); err != nil {
			return err
		}
		return writeNodeList(w, node.Body)

	case *BreakStmt:
		return writeByte(w, byte(KindBreakStmt))

	case *ContinueStmt:
		return writeByte(w, byte(KindContinueStmt))

	case *ExprStmt:
		if err := writeByte(w, byte(KindExprStmt)); err != nil {
			return err
		}
		return encodeNode(w, node.Expr)

	case *StructDecl:
		if err := writeByte(w, byte(KindStructDecl)); err != nil {
			return err
		}
		if err := writeString(w, node.Name); err != nil {
			return err
		}
		if len(node.Fields) > MaxChildCount {
			return fmt.Errorf("XQL_E413: field count %d > %d", len(node.Fields), MaxChildCount)
		}
		if err := binary.Write(w, binary.BigEndian, uint32(len(node.Fields))); err != nil {
			return err
		}
		for _, f := range node.Fields {
			if err := writeStructField(w, f); err != nil {
				return err
			}
		}
		return nil

	case *EnumDecl:
		if err := writeByte(w, byte(KindEnumDecl)); err != nil {
			return err
		}
		if err := writeString(w, node.Name); err != nil {
			return err
		}
		return writeStringList(w, node.Variants)

	case *MatchExpr:
		if err := writeByte(w, byte(KindMatchExpr)); err != nil {
			return err
		}
		if err := encodeNode(w, node.Value); err != nil {
			return err
		}
		if len(node.Arms) > MaxChildCount {
			return fmt.Errorf("XQL_E413: arm count %d > %d", len(node.Arms), MaxChildCount)
		}
		if err := binary.Write(w, binary.BigEndian, uint32(len(node.Arms))); err != nil {
			return err
		}
		for _, arm := range node.Arms {
			if err := writeMatchArm(w, arm); err != nil {
				return err
			}
		}
		return nil

	case *BinaryExpr:
		if err := writeByte(w, byte(KindBinaryExpr)); err != nil {
			return err
		}
		if err := writeString(w, node.Op); err != nil {
			return err
		}
		if err := encodeNode(w, node.Left); err != nil {
			return err
		}
		return encodeNode(w, node.Right)

	case *UnaryExpr:
		if err := writeByte(w, byte(KindUnaryExpr)); err != nil {
			return err
		}
		if err := writeString(w, node.Op); err != nil {
			return err
		}
		return encodeNode(w, node.Operand)

	case *CallExpr:
		if err := writeByte(w, byte(KindCallExpr)); err != nil {
			return err
		}
		if err := writeString(w, node.Callee); err != nil {
			return err
		}
		return writeNodeList(w, node.Args)

	case *Literal:
		if err := writeByte(w, byte(KindLiteral)); err != nil {
			return err
		}
		if err := writeString(w, node.ValueType); err != nil {
			return err
		}
		return writeLiteralValue(w, node.Value)

	case *Ident:
		if err := writeByte(w, byte(KindIdent)); err != nil {
			return err
		}
		return writeString(w, node.Name)

	case *MemberExpr:
		if err := writeByte(w, byte(KindMemberExpr)); err != nil {
			return err
		}
		if err := writeString(w, node.Field); err != nil {
			return err
		}
		return encodeNode(w, node.Object)

	case *StructLit:
		if err := writeByte(w, byte(KindStructLit)); err != nil {
			return err
		}
		if err := writeString(w, node.TypeName); err != nil {
			return err
		}
		if len(node.Fields) > MaxChildCount {
			return fmt.Errorf("XQL_E413: field count %d > %d", len(node.Fields), MaxChildCount)
		}
		if err := binary.Write(w, binary.BigEndian, uint32(len(node.Fields))); err != nil {
			return err
		}
		for _, fi := range node.Fields {
			if err := writeStructFieldInit(w, fi); err != nil {
				return err
			}
		}
		return nil

	case *ArrayLit:
		if err := writeByte(w, byte(KindArrayLit)); err != nil {
			return err
		}
		if err := writeTypeExpr(w, node.ElemType); err != nil {
			return err
		}
		return writeNodeList(w, node.Elements)

	case *IndexExpr:
		if err := writeByte(w, byte(KindIndexExpr)); err != nil {
			return err
		}
		if err := encodeNode(w, node.Target); err != nil {
			return err
		}
		return encodeNode(w, node.Index)

	case *IfExpr:
		if err := writeByte(w, byte(KindIfExpr)); err != nil {
			return err
		}
		if err := encodeNode(w, node.Cond); err != nil {
			return err
		}
		if err := encodeNode(w, node.Then); err != nil {
			return err
		}
		return encodeNode(w, node.Else)

	case *NewExpr:
		if err := writeByte(w, byte(KindNewExpr)); err != nil {
			return err
		}
		if err := writeString(w, node.Callee); err != nil {
			return err
		}
		return writeNodeList(w, node.Args)

	case *AwaitExpr:
		if err := writeByte(w, byte(KindAwaitExpr)); err != nil {
			return err
		}
		return encodeNode(w, node.Expr)

	case *Lambda:
		if err := writeByte(w, byte(KindLambda)); err != nil {
			return err
		}
		if len(node.Params) > MaxChildCount {
			return fmt.Errorf("XQL_E413: param count %d > %d", len(node.Params), MaxChildCount)
		}
		if err := binary.Write(w, binary.BigEndian, uint32(len(node.Params))); err != nil {
			return err
		}
		for _, p := range node.Params {
			if err := writeParam(w, p); err != nil {
				return err
			}
		}
		if err := writeTypeExpr(w, node.ReturnType); err != nil {
			return err
		}
		return writeNodeList(w, node.Body)

	case *ClassDecl:
		if err := writeByte(w, byte(KindClassDecl)); err != nil {
			return err
		}
		if err := writeString(w, node.Name); err != nil {
			return err
		}
		if len(node.Fields) > MaxChildCount {
			return fmt.Errorf("XQL_E413: field count %d > %d", len(node.Fields), MaxChildCount)
		}
		if err := binary.Write(w, binary.BigEndian, uint32(len(node.Fields))); err != nil {
			return err
		}
		for _, f := range node.Fields {
			if err := writeClassField(w, f); err != nil {
				return err
			}
		}
		return nil

	case *SwitchStmt:
		if err := writeByte(w, byte(KindSwitchStmt)); err != nil {
			return err
		}
		if err := encodeNode(w, node.Value); err != nil {
			return err
		}
		if len(node.Cases) > MaxChildCount {
			return fmt.Errorf("XQL_E413: case count %d > %d", len(node.Cases), MaxChildCount)
		}
		if err := binary.Write(w, binary.BigEndian, uint32(len(node.Cases))); err != nil {
			return err
		}
		for _, c := range node.Cases {
			if err := writeSwitchCase(w, c); err != nil {
				return err
			}
		}
		return nil

	case *MapLiteral:
		if err := writeByte(w, byte(KindMapLiteral)); err != nil {
			return err
		}
		if err := writeTypeExpr(w, node.KeyType); err != nil {
			return err
		}
		if err := writeTypeExpr(w, node.ValueType); err != nil {
			return err
		}
		if len(node.Entries) > MaxChildCount {
			return fmt.Errorf("XQL_E413: entry count %d > %d", len(node.Entries), MaxChildCount)
		}
		if err := binary.Write(w, binary.BigEndian, uint32(len(node.Entries))); err != nil {
			return err
		}
		for _, e := range node.Entries {
			if err := writeMapEntry(w, e); err != nil {
				return err
			}
		}
		return nil

	case *ArrayLiteral:
		if err := writeByte(w, byte(KindArrayLiteral)); err != nil {
			return err
		}
		if err := writeTypeExpr(w, node.ElemType); err != nil {
			return err
		}
		return writeNodeList(w, node.Elements)

	case *ImportDecl:
		if err := writeByte(w, byte(KindImportDecl)); err != nil {
			return err
		}
		if err := writeString(w, node.Path); err != nil {
			return err
		}
		return writeString(w, node.As)

	default:
		return fmt.Errorf("unknown node type for serialization: %T", n)
	}
}

func decodeNode(r io.Reader, depth int) (Node, error) {
	// 递归深度检查，防止恶意嵌套导致栈溢出
	if depth > MaxDecodeDepth {
		return nil, fmt.Errorf("XQL_E413: decode recursion depth %d > %d", depth, MaxDecodeDepth)
	}
	kindByte, err := readByte(r)
	if err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	if kindByte == 0 {
		return nil, nil
	}

	switch NodeKind(kindByte) {
	case KindProgram:
		decls, err := readNodeList(r, depth)
		if err != nil {
			return nil, err
		}
		return &Program{Decls: decls}, nil

	case KindFunctionDecl:
		fd := &FunctionDecl{}
		name, err := readString(r)
		if err != nil {
			return nil, err
		}
		fd.Name = name
		var numParams uint32
		if err := binary.Read(r, binary.BigEndian, &numParams); err != nil {
			return nil, err
		}
		if numParams > MaxChildCount {
			return nil, fmt.Errorf("XQL_E413: param count %d > %d", numParams, MaxChildCount)
		}
		if numParams > 0 {
			fd.Params = make([]Param, numParams)
			for i := uint32(0); i < numParams; i++ {
				p, err := readParam(r)
				if err != nil {
					return nil, err
				}
				fd.Params[i] = p
			}
		} else {
			fd.Params = nil
		}
		retType, err := readTypeExpr(r)
		if err != nil {
			return nil, err
		}
		fd.ReturnType = retType
		effects, err := readStringList(r)
		if err != nil {
			return nil, err
		}
		fd.Effects = effects
		grants, err := readStringList(r)
		if err != nil {
			return nil, err
		}
		fd.Grant = grants
		body, err := readNodeList(r, depth)
		if err != nil {
			return nil, err
		}
		fd.Body = body
		return fd, nil

	case KindReturnStmt:
		val, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		return &ReturnStmt{Value: val}, nil

	case KindVarDecl:
		vd := &VarDecl{}
		name, err := readString(r)
		if err != nil {
			return nil, err
		}
		vd.Name = name
		t, err := readTypeExpr(r)
		if err != nil {
			return nil, err
		}
		vd.Type = t
		val, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		vd.Value = val
		return vd, nil

	case KindAssignStmt:
		as := &AssignStmt{}
		target, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		as.Target = target
		val, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		as.Value = val
		return as, nil

	case KindIfStmt:
		is := &IfStmt{}
		cond, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		is.Cond = cond
		thenNodes, err := readNodeList(r, depth)
		if err != nil {
			return nil, err
		}
		is.Then = thenNodes
		elseNodes, err := readNodeList(r, depth)
		if err != nil {
			return nil, err
		}
		is.Else = elseNodes
		return is, nil

	case KindWhileStmt:
		ws := &WhileStmt{}
		cond, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		ws.Cond = cond
		bodyNodes, err := readNodeList(r, depth)
		if err != nil {
			return nil, err
		}
		ws.Body = bodyNodes
		return ws, nil

	case KindForStmt:
		fs := &ForStmt{}
		form, err := readString(r)
		if err != nil {
			return nil, err
		}
		fs.Form = form
		varName, err := readString(r)
		if err != nil {
			return nil, err
		}
		fs.Var = varName
		start, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		fs.Start = start
		end, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		fs.End = end
		iterable, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		fs.Iterable = iterable
		bodyNodes, err := readNodeList(r, depth)
		if err != nil {
			return nil, err
		}
		fs.Body = bodyNodes
		return fs, nil

	case KindBreakStmt:
		return &BreakStmt{}, nil

	case KindContinueStmt:
		return &ContinueStmt{}, nil

	case KindExprStmt:
		es := &ExprStmt{}
		expr, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		es.Expr = expr
		return es, nil

	case KindStructDecl:
		sd := &StructDecl{}
		name, err := readString(r)
		if err != nil {
			return nil, err
		}
		sd.Name = name
		var numFields uint32
		if err := binary.Read(r, binary.BigEndian, &numFields); err != nil {
			return nil, err
		}
		if numFields > MaxChildCount {
			return nil, fmt.Errorf("XQL_E413: field count %d > %d", numFields, MaxChildCount)
		}
		if numFields > 0 {
			sd.Fields = make([]StructField, numFields)
			for i := uint32(0); i < numFields; i++ {
				f, err := readStructField(r, depth)
				if err != nil {
					return nil, err
				}
				sd.Fields[i] = f
			}
		} else {
			sd.Fields = nil
		}
		return sd, nil

	case KindEnumDecl:
		ed := &EnumDecl{}
		name, err := readString(r)
		if err != nil {
			return nil, err
		}
		ed.Name = name
		variants, err := readStringList(r)
		if err != nil {
			return nil, err
		}
		ed.Variants = variants
		return ed, nil

	case KindMatchExpr:
		me := &MatchExpr{}
		val, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		me.Value = val
		var numArms uint32
		if err := binary.Read(r, binary.BigEndian, &numArms); err != nil {
			return nil, err
		}
		if numArms > MaxChildCount {
			return nil, fmt.Errorf("XQL_E413: arm count %d > %d", numArms, MaxChildCount)
		}
		if numArms > 0 {
			me.Arms = make([]MatchArm, numArms)
			for i := uint32(0); i < numArms; i++ {
				arm, err := readMatchArm(r, depth)
				if err != nil {
					return nil, err
				}
				me.Arms[i] = arm
			}
		} else {
			me.Arms = nil
		}
		return me, nil

	case KindBinaryExpr:
		be := &BinaryExpr{}
		op, err := readString(r)
		if err != nil {
			return nil, err
		}
		be.Op = op
		left, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		be.Left = left
		right, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		be.Right = right
		return be, nil

	case KindUnaryExpr:
		ue := &UnaryExpr{}
		op, err := readString(r)
		if err != nil {
			return nil, err
		}
		ue.Op = op
		operand, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		ue.Operand = operand
		return ue, nil

	case KindCallExpr:
		ce := &CallExpr{}
		callee, err := readString(r)
		if err != nil {
			return nil, err
		}
		ce.Callee = callee
		args, err := readNodeList(r, depth)
		if err != nil {
			return nil, err
		}
		ce.Args = args
		return ce, nil

	case KindLiteral:
		l := &Literal{}
		valueType, err := readString(r)
		if err != nil {
			return nil, err
		}
		l.ValueType = valueType
		value, err := readLiteralValue(r)
		if err != nil {
			return nil, err
		}
		l.Value = value
		return l, nil

	case KindIdent:
		id := &Ident{}
		name, err := readString(r)
		if err != nil {
			return nil, err
		}
		id.Name = name
		return id, nil

	case KindMemberExpr:
		me := &MemberExpr{}
		field, err := readString(r)
		if err != nil {
			return nil, err
		}
		me.Field = field
		obj, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		me.Object = obj
		return me, nil

	case KindStructLit:
		sl := &StructLit{}
		typeName, err := readString(r)
		if err != nil {
			return nil, err
		}
		sl.TypeName = typeName
		var numFields uint32
		if err := binary.Read(r, binary.BigEndian, &numFields); err != nil {
			return nil, err
		}
		if numFields > MaxChildCount {
			return nil, fmt.Errorf("XQL_E413: field count %d > %d", numFields, MaxChildCount)
		}
		if numFields > 0 {
			sl.Fields = make([]StructFieldInit, numFields)
			for i := uint32(0); i < numFields; i++ {
				fi, err := readStructFieldInit(r, depth)
				if err != nil {
					return nil, err
				}
				sl.Fields[i] = fi
			}
		} else {
			sl.Fields = nil
		}
		return sl, nil

	case KindArrayLit:
		al := &ArrayLit{}
		elemType, err := readTypeExpr(r)
		if err != nil {
			return nil, err
		}
		al.ElemType = elemType
		elements, err := readNodeList(r, depth)
		if err != nil {
			return nil, err
		}
		al.Elements = elements
		return al, nil

	case KindIndexExpr:
		ie := &IndexExpr{}
		target, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		ie.Target = target
		index, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		ie.Index = index
		return ie, nil

	case KindIfExpr:
		ie := &IfExpr{}
		cond, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		ie.Cond = cond
		thenBranch, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		ie.Then = thenBranch
		elseBranch, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		ie.Else = elseBranch
		return ie, nil

	case KindNewExpr:
		ne := &NewExpr{}
		callee, err := readString(r)
		if err != nil {
			return nil, err
		}
		ne.Callee = callee
		args, err := readNodeList(r, depth)
		if err != nil {
			return nil, err
		}
		ne.Args = args
		return ne, nil

	case KindAwaitExpr:
		ae := &AwaitExpr{}
		expr, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		ae.Expr = expr
		return ae, nil

	case KindLambda:
		l := &Lambda{}
		var numParams uint32
		if err := binary.Read(r, binary.BigEndian, &numParams); err != nil {
			return nil, err
		}
		if numParams > MaxChildCount {
			return nil, fmt.Errorf("XQL_E413: param count %d > %d", numParams, MaxChildCount)
		}
		if numParams > 0 {
			l.Params = make([]Param, numParams)
			for i := uint32(0); i < numParams; i++ {
				p, err := readParam(r)
				if err != nil {
					return nil, err
				}
				l.Params[i] = p
			}
		} else {
			l.Params = nil
		}
		retType, err := readTypeExpr(r)
		if err != nil {
			return nil, err
		}
		l.ReturnType = retType
		body, err := readNodeList(r, depth)
		if err != nil {
			return nil, err
		}
		l.Body = body
		return l, nil

	case KindClassDecl:
		cd := &ClassDecl{}
		name, err := readString(r)
		if err != nil {
			return nil, err
		}
		cd.Name = name
		var numFields uint32
		if err := binary.Read(r, binary.BigEndian, &numFields); err != nil {
			return nil, err
		}
		if numFields > MaxChildCount {
			return nil, fmt.Errorf("XQL_E413: field count %d > %d", numFields, MaxChildCount)
		}
		if numFields > 0 {
			cd.Fields = make([]ClassField, numFields)
			for i := uint32(0); i < numFields; i++ {
				f, err := readClassField(r, depth)
				if err != nil {
					return nil, err
				}
				cd.Fields[i] = f
			}
		} else {
			cd.Fields = nil
		}
		return cd, nil

	case KindSwitchStmt:
		ss := &SwitchStmt{}
		val, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		ss.Value = val
		var numCases uint32
		if err := binary.Read(r, binary.BigEndian, &numCases); err != nil {
			return nil, err
		}
		if numCases > MaxChildCount {
			return nil, fmt.Errorf("XQL_E413: case count %d > %d", numCases, MaxChildCount)
		}
		if numCases > 0 {
			ss.Cases = make([]SwitchCase, numCases)
			for i := uint32(0); i < numCases; i++ {
				c, err := readSwitchCase(r, depth)
				if err != nil {
					return nil, err
				}
				ss.Cases[i] = c
			}
		} else {
			ss.Cases = nil
		}
		return ss, nil

	case KindMapLiteral:
		ml := &MapLiteral{}
		keyType, err := readTypeExpr(r)
		if err != nil {
			return nil, err
		}
		ml.KeyType = keyType
		valueType, err := readTypeExpr(r)
		if err != nil {
			return nil, err
		}
		ml.ValueType = valueType
		var numEntries uint32
		if err := binary.Read(r, binary.BigEndian, &numEntries); err != nil {
			return nil, err
		}
		if numEntries > MaxChildCount {
			return nil, fmt.Errorf("XQL_E413: entry count %d > %d", numEntries, MaxChildCount)
		}
		if numEntries > 0 {
			ml.Entries = make([]MapEntry, numEntries)
			for i := uint32(0); i < numEntries; i++ {
				e, err := readMapEntry(r, depth)
				if err != nil {
					return nil, err
				}
				ml.Entries[i] = e
			}
		} else {
			ml.Entries = nil
		}
		return ml, nil

	case KindArrayLiteral:
		al := &ArrayLiteral{}
		elemType, err := readTypeExpr(r)
		if err != nil {
			return nil, err
		}
		al.ElemType = elemType
		elements, err := readNodeList(r, depth)
		if err != nil {
			return nil, err
		}
		al.Elements = elements
		return al, nil

	case KindImportDecl:
		path, err := readString(r)
		if err != nil {
			return nil, err
		}
		as, err := readString(r)
		if err != nil {
			return nil, err
		}
		return &ImportDecl{Path: path, As: as}, nil

	default:
		return nil, fmt.Errorf("unknown binary kind byte: %d", kindByte)
	}
}

// Helper serialization functions

func writeByte(w io.Writer, b byte) error {
	return binary.Write(w, binary.BigEndian, b)
}

func readByte(r io.Reader) (byte, error) {
	var b byte
	err := binary.Read(r, binary.BigEndian, &b)
	return b, err
}

func writeString(w io.Writer, s string) error {
	data := []byte(s)
	if err := binary.Write(w, binary.BigEndian, uint32(len(data))); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func readString(r io.Reader) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", err
	}
	if length > MaxStringLen {
		return "", fmt.Errorf("XQL_E413: string length %d > %d", length, MaxStringLen)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", err
	}
	return string(data), nil
}

func writeBool(w io.Writer, b bool) error {
	var val byte
	if b {
		val = 1
	}
	return writeByte(w, val)
}

func readBool(r io.Reader) (bool, error) {
	val, err := readByte(r)
	return val != 0, err
}

func writeTypeExpr(w io.Writer, te TypeExpr) error {
	if err := writeString(w, te.KindName); err != nil {
		return err
	}
	if te.Elem != nil {
		if err := writeBool(w, true); err != nil {
			return err
		}
		if err := writeTypeExpr(w, *te.Elem); err != nil {
			return err
		}
	} else {
		if err := writeBool(w, false); err != nil {
			return err
		}
	}
	if te.KeyType != nil {
		if err := writeBool(w, true); err != nil {
			return err
		}
		if err := writeTypeExpr(w, *te.KeyType); err != nil {
			return err
		}
	} else {
		if err := writeBool(w, false); err != nil {
			return err
		}
	}
	if te.OkType != nil {
		if err := writeBool(w, true); err != nil {
			return err
		}
		if err := writeTypeExpr(w, *te.OkType); err != nil {
			return err
		}
	} else {
		if err := writeBool(w, false); err != nil {
			return err
		}
	}
	if te.ErrType != nil {
		if err := writeBool(w, true); err != nil {
			return err
		}
		if err := writeTypeExpr(w, *te.ErrType); err != nil {
			return err
		}
	} else {
		if err := writeBool(w, false); err != nil {
			return err
		}
	}
	return nil
}

func readTypeExpr(r io.Reader) (TypeExpr, error) {
	te := TypeExpr{}
	kindName, err := readString(r)
	if err != nil {
		return te, err
	}
	te.KindName = kindName

	hasElem, err := readBool(r)
	if err != nil {
		return te, err
	}
	if hasElem {
		elem, err := readTypeExpr(r)
		if err != nil {
			return te, err
		}
		te.Elem = &elem
	}

	hasKeyType, err := readBool(r)
	if err != nil {
		return te, err
	}
	if hasKeyType {
		keyType, err := readTypeExpr(r)
		if err != nil {
			return te, err
		}
		te.KeyType = &keyType
	}

	hasOkType, err := readBool(r)
	if err != nil {
		return te, err
	}
	if hasOkType {
		okType, err := readTypeExpr(r)
		if err != nil {
			return te, err
		}
		te.OkType = &okType
	}

	hasErrType, err := readBool(r)
	if err != nil {
		return te, err
	}
	if hasErrType {
		errType, err := readTypeExpr(r)
		if err != nil {
			return te, err
		}
		te.ErrType = &errType
	}

	return te, nil
}

func writeParam(w io.Writer, p Param) error {
	if err := writeString(w, p.Name); err != nil {
		return err
	}
	return writeTypeExpr(w, p.Type)
}

func readParam(r io.Reader) (Param, error) {
	p := Param{}
	name, err := readString(r)
	if err != nil {
		return p, err
	}
	p.Name = name
	t, err := readTypeExpr(r)
	if err != nil {
		return p, err
	}
	p.Type = t
	return p, nil
}

func writeStructField(w io.Writer, sf StructField) error {
	if err := writeString(w, sf.Name); err != nil {
		return err
	}
	if err := writeTypeExpr(w, sf.Type); err != nil {
		return err
	}
	return writeString(w, sf.Visibility)
}

func readStructField(r io.Reader, depth int) (StructField, error) {
	sf := StructField{}
	name, err := readString(r)
	if err != nil {
		return sf, err
	}
	sf.Name = name
	t, err := readTypeExpr(r)
	if err != nil {
		return sf, err
	}
	sf.Type = t
	vis, err := readString(r)
	if err != nil {
		return sf, err
	}
	sf.Visibility = vis
	return sf, nil
}

func writeClassField(w io.Writer, cf ClassField) error {
	if err := writeString(w, cf.Name); err != nil {
		return err
	}
	if err := writeTypeExpr(w, cf.Type); err != nil {
		return err
	}
	return writeString(w, cf.Visibility)
}

func readClassField(r io.Reader, depth int) (ClassField, error) {
	cf := ClassField{}
	name, err := readString(r)
	if err != nil {
		return cf, err
	}
	cf.Name = name
	t, err := readTypeExpr(r)
	if err != nil {
		return cf, err
	}
	cf.Type = t
	vis, err := readString(r)
	if err != nil {
		return cf, err
	}
	cf.Visibility = vis
	return cf, nil
}

func writeStructFieldInit(w io.Writer, sfi StructFieldInit) error {
	if err := writeString(w, sfi.Name); err != nil {
		return err
	}
	return encodeNode(w, sfi.Value)
}

func readStructFieldInit(r io.Reader, depth int) (StructFieldInit, error) {
	sfi := StructFieldInit{}
	name, err := readString(r)
	if err != nil {
		return sfi, err
	}
	sfi.Name = name
	val, err := decodeNode(r, depth+1)
	if err != nil {
		return sfi, err
	}
	sfi.Value = val
	return sfi, nil
}

func writeMatchArm(w io.Writer, arm MatchArm) error {
	if arm.Pattern != nil {
		if err := writeBool(w, true); err != nil {
			return err
		}
		if err := encodeNode(w, arm.Pattern); err != nil {
			return err
		}
	} else {
		if err := writeBool(w, false); err != nil {
			return err
		}
	}
	return writeNodeList(w, arm.Body)
}

func readMatchArm(r io.Reader, depth int) (MatchArm, error) {
	arm := MatchArm{}
	hasPattern, err := readBool(r)
	if err != nil {
		return arm, err
	}
	if hasPattern {
		pattern, err := decodeNode(r, depth+1)
		if err != nil {
			return arm, err
		}
		arm.Pattern = pattern
	}
	body, err := readNodeList(r, depth)
	if err != nil {
		return arm, err
	}
	arm.Body = body
	return arm, nil
}

func writeSwitchCase(w io.Writer, sc SwitchCase) error {
	if sc.Value != nil {
		if err := writeBool(w, true); err != nil {
			return err
		}
		if err := encodeNode(w, sc.Value); err != nil {
			return err
		}
	} else {
		if err := writeBool(w, false); err != nil {
			return err
		}
	}
	return writeNodeList(w, sc.Body)
}

func readSwitchCase(r io.Reader, depth int) (SwitchCase, error) {
	sc := SwitchCase{}
	hasValue, err := readBool(r)
	if err != nil {
		return sc, err
	}
	if hasValue {
		val, err := decodeNode(r, depth+1)
		if err != nil {
			return sc, err
		}
		sc.Value = val
	}
	body, err := readNodeList(r, depth)
	if err != nil {
		return sc, err
	}
	sc.Body = body
	return sc, nil
}

func writeMapEntry(w io.Writer, me MapEntry) error {
	if err := encodeNode(w, me.Key); err != nil {
		return err
	}
	return encodeNode(w, me.Value)
}

func readMapEntry(r io.Reader, depth int) (MapEntry, error) {
	me := MapEntry{}
	k, err := decodeNode(r, depth+1)
	if err != nil {
		return me, err
	}
	me.Key = k
	v, err := decodeNode(r, depth+1)
	if err != nil {
		return me, err
	}
	me.Value = v
	return me, nil
}

func writeNodeList(w io.Writer, list []Node) error {
	if len(list) > MaxChildCount {
		return fmt.Errorf("XQL_E413: child count %d > %d", len(list), MaxChildCount)
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(list))); err != nil {
		return err
	}
	for _, item := range list {
		if err := encodeNode(w, item); err != nil {
			return err
		}
	}
	return nil
}

func readNodeList(r io.Reader, depth int) ([]Node, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if length > MaxChildCount {
		return nil, fmt.Errorf("XQL_E413: child count %d > %d", length, MaxChildCount)
	}
	if length == 0 {
		return nil, nil
	}
	list := make([]Node, length)
	for i := uint32(0); i < length; i++ {
		n, err := decodeNode(r, depth+1)
		if err != nil {
			return nil, err
		}
		list[i] = n
	}
	return list, nil
}

func writeStringList(w io.Writer, list []string) error {
	if err := binary.Write(w, binary.BigEndian, uint32(len(list))); err != nil {
		return err
	}
	for _, s := range list {
		if err := writeString(w, s); err != nil {
			return err
		}
	}
	return nil
}

func readStringList(r io.Reader) ([]string, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if length > MaxChildCount {
		return nil, fmt.Errorf("XQL_E413: string list length %d > %d", length, MaxChildCount)
	}
	if length == 0 {
		return nil, nil
	}
	list := make([]string, length)
	for i := uint32(0); i < length; i++ {
		s, err := readString(r)
		if err != nil {
			return nil, err
		}
		list[i] = s
	}
	return list, nil
}

func writeLiteralValue(w io.Writer, val interface{}) error {
	if val == nil {
		return writeByte(w, 0)
	}
	switch v := val.(type) {
	case string:
		if err := writeByte(w, 1); err != nil {
			return err
		}
		return writeString(w, v)
	case float64:
		if err := writeByte(w, 2); err != nil {
			return err
		}
		return binary.Write(w, binary.BigEndian, v)
	case int64:
		if err := writeByte(w, 3); err != nil {
			return err
		}
		return binary.Write(w, binary.BigEndian, v)
	case int:
		if err := writeByte(w, 4); err != nil {
			return err
		}
		return binary.Write(w, binary.BigEndian, int64(v))
	case bool:
		if err := writeByte(w, 5); err != nil {
			return err
		}
		return writeBool(w, v)
	default:
		return fmt.Errorf("unsupported literal value type: %T", val)
	}
}

func readLiteralValue(r io.Reader) (interface{}, error) {
	t, err := readByte(r)
	if err != nil {
		return nil, err
	}
	switch t {
	case 0:
		return nil, nil
	case 1:
		return readString(r)
	case 2:
		var v float64
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 3:
		var v int64
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 4:
		var v int64
		err := binary.Read(r, binary.BigEndian, &v)
		return int(v), err
	case 5:
		return readBool(r)
	default:
		return nil, fmt.Errorf("unsupported literal value type byte: %d", t)
	}
}
