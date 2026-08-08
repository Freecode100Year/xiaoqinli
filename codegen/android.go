package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateAndroidProject produces a complete Android Gradle project structure from the AST.
func GenerateAndroidProject(root ast.Node) (*ProjectOutput, error) {
	ktCode, err := GenerateAndroidKotlin(root)
	if err != nil {
		return nil, err
	}

	files := map[string][]byte{
		"build.gradle":                                  []byte(getAndroidRootBuildGradle()),
		"settings.gradle":                               []byte(getAndroidSettingsGradle()),
		"gradle.properties":                             []byte(getAndroidGradleProperties()),
		"app/build.gradle":                              []byte(getAndroidAppBuildGradle()),
		"app/src/main/AndroidManifest.xml":              []byte(getAndroidManifest()),
		"app/src/main/res/layout/activity_main.xml":     []byte(getAndroidLayoutXml()),
		"app/src/main/res/values/strings.xml":           []byte(getAndroidStringsXml()),
		"app/src/main/java/com/xql/app/MainActivity.kt": ktCode,
	}

	return &ProjectOutput{
		MainCode: ktCode,
		Files:    files,
	}, nil
}

// GenerateAndroidKotlin produces the MainActivity.kt logic for Android target.
func GenerateAndroidKotlin(root ast.Node) ([]byte, error) {
	g := &androidGen{
		buf:  &strings.Builder{},
		muts: map[string]bool{},
	}
	return g.generate(root)
}

type androidGen struct {
	buf    *strings.Builder
	indent int
	muts   map[string]bool
}

func (g *androidGen) generate(root ast.Node) ([]byte, error) {
	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	g.writeln("package com.xql.app")
	g.writeln("")
	g.writeln("import android.os.Bundle")
	g.writeln("import android.widget.TextView")
	g.writeln("import androidx.appcompat.app.AppCompatActivity")
	g.writeln("")

	// Emit top-level Structs / Classes / Enums outside MainActivity
	for _, d := range prog.Decls {
		switch node := d.(type) {
		case *ast.StructDecl:
			if err := g.emitStructDecl(node); err != nil {
				return nil, err
			}
			g.writeln("")
		case *ast.ClassDecl:
			if err := g.emitClassDecl(node); err != nil {
				return nil, err
			}
			g.writeln("")
		case *ast.EnumDecl:
			if err := g.emitEnumDecl(node); err != nil {
				return nil, err
			}
			g.writeln("")
		}
	}

	g.writeln("class MainActivity : AppCompatActivity() {")
	g.indent++
	g.writeln("private lateinit var tvOutput: TextView")
	g.writeln("")
	g.writeln("override fun onCreate(savedInstanceState: Bundle?) {")
	g.indent++
	g.writeln("super.onCreate(savedInstanceState)")
	g.writeln("setContentView(R.layout.activity_main)")
	g.writeln("tvOutput = findViewById(R.id.tvOutput)")
	g.writeln("runXqlApp()")
	g.indent--
	g.writeln("}")
	g.writeln("")

	g.writeln("private fun println(msg: Any?) {")
	g.indent++
	g.writeln("val text = msg?.toString() ?: \"null\"")
	g.writeln("tvOutput.append(text + \"\\n\")")
	g.writeln("android.util.Log.d(\"XQL\", text)")
	g.indent--
	g.writeln("}")
	g.writeln("")

	g.writeln("private fun runXqlApp() {")
	g.indent++

	// Collect mutables and emit statements inside main
	for _, d := range prog.Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok && fd.Name == "main" {
			g.muts = collectMutables(fd.Body)
			for _, stmt := range fd.Body {
				if err := g.emitNode(stmt); err != nil {
					return nil, err
				}
			}
		}
	}

	g.indent--
	g.writeln("}")

	// Emit non-main helper functions inside MainActivity
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name == "main" {
			continue
		}
		g.writeln("")
		if err := g.emitFunctionDecl(fd); err != nil {
			return nil, err
		}
	}

	g.indent--
	g.writeln("}")

	return []byte(g.buf.String()), nil
}

func (g *androidGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.write("data class " + sd.Name + "(")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write("val " + f.Name + ": " + typeToKotlin(f.Type))
	}
	g.writeln(")")
	return nil
}

func (g *androidGen) emitClassDecl(cd *ast.ClassDecl) error {
	g.writeIndent()
	g.writeln("public class " + cd.Name + " {")
	g.indent++
	for _, f := range cd.Fields {
		g.writeIndent()
		vis := f.Visibility
		if vis == "" {
			vis = "public"
		}
		g.writeln(vis + " var " + f.Name + ": " + typeToKotlin(f.Type) + " = null")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *androidGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.write("enum class " + ed.Name + " { ")
	for i, v := range ed.Variants {
		if i > 0 {
			g.write(", ")
		}
		g.write(v)
	}
	g.writeln(" }")
	return nil
}

func (g *androidGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	prevMuts := g.muts
	g.muts = collectMutables(fd.Body)
	defer func() { g.muts = prevMuts }()

	g.writeIndent()
	g.write(fmt.Sprintf("private fun %s(", fd.Name))
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(fmt.Sprintf("%s: %s", p.Name, typeToKotlin(p.Type)))
	}
	g.write(")")
	rt := typeToKotlin(fd.ReturnType)
	if rt != "" && rt != "Unit" && rt != "Any" {
		g.write(": " + rt)
	}
	g.writeln(" {")
	g.indent++
	for _, s := range fd.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *androidGen) emitNode(n ast.Node) error {
	switch node := n.(type) {
	case *ast.VarDecl:
		return g.emitVarDecl(node)
	case *ast.AssignStmt:
		return g.emitAssign(node)
	case *ast.ExprStmt:
		return g.emitExprStmt(node)
	case *ast.IfStmt:
		return g.emitIf(node)
	case *ast.WhileStmt:
		return g.emitWhile(node)
	case *ast.ForStmt:
		return g.emitFor(node)
	case *ast.ReturnStmt:
		return g.emitReturn(node)
	case *ast.BreakStmt:
		g.writeIndent()
		g.writeln("break")
		return nil
	case *ast.ContinueStmt:
		g.writeIndent()
		g.writeln("continue")
		return nil
	case *ast.SwitchStmt:
		return g.emitSwitch(node)
	default:
		return g.emitExprStmt(&ast.ExprStmt{Expr: node})
	}
}

func (g *androidGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	kw := "val"
	if g.muts[vd.Name] {
		kw = "var"
	}
	g.write(fmt.Sprintf("%s %s", kw, vd.Name))
	if vd.Type.KindName != "" {
		g.write(fmt.Sprintf(": %s", typeToKotlin(vd.Type)))
	}
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln("")
	return nil
}

func (g *androidGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	if err := g.emitExpr(as.Target); err != nil {
		return err
	}
	g.write(" = ")
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *androidGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *androidGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if (")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(") {")
	g.indent++
	for _, s := range is.Then {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	if len(is.Else) > 0 {
		g.writeln("} else {")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
	}
	g.writeln("}")
	return nil
}

func (g *androidGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while (")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(") {")
	g.indent++
	for _, s := range ws.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *androidGen) emitFor(fs *ast.ForStmt) error {
	g.writeIndent()
	if fs.Form == "range" {
		g.write(fmt.Sprintf("for (%s in ", fs.Var))
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("..")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(") {")
	} else {
		g.write(fmt.Sprintf("for (%s in ", fs.Var))
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(") {")
	}
	g.indent++
	for _, s := range fs.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *androidGen) emitSwitch(ss *ast.SwitchStmt) error {
	g.writeIndent()
	g.write("when (")
	if err := g.emitExpr(ss.Value); err != nil {
		return err
	}
	g.writeln(") {")
	g.indent++
	for _, c := range ss.Cases {
		g.writeIndent()
		if c.Value != nil {
			if err := g.emitExpr(c.Value); err != nil {
				return err
			}
		} else {
			g.write("else")
		}
		g.writeln(" -> {")
		g.indent++
		for _, stmt := range c.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("}")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *androidGen) emitReturn(rs *ast.ReturnStmt) error {
	g.writeIndent()
	g.write("return ")
	if rs.Value != nil {
		if err := g.emitExpr(rs.Value); err != nil {
			return err
		}
	}
	g.writeln("")
	return nil
}

func (g *androidGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		if node.ValueType == "String" {
			strVal, _ := node.Value.(string)
			g.write(fmt.Sprintf("%q", strVal))
		} else if strVal, ok := node.Value.(string); ok {
			g.write(fmt.Sprintf("%q", strVal))
		} else if f, ok := node.Value.(float64); ok && node.ValueType == "Int" {
			g.write(fmt.Sprintf("%dL", int64(f)))
		} else {
			g.write(fmt.Sprintf("%v", node.Value))
		}
		return nil
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		g.write("(")
		if err := g.emitExpr(node.Left); err != nil {
			return err
		}
		g.write(" " + node.Op + " ")
		if err := g.emitExpr(node.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	case *ast.UnaryExpr:
		g.write(node.Op)
		return g.emitExpr(node.Operand)
	case *ast.MemberExpr:
		if err := g.emitExpr(node.Object); err != nil {
			return err
		}
		g.write("." + node.Field)
		return nil
	case *ast.IndexExpr:
		if err := g.emitExpr(node.Target); err != nil {
			return err
		}
		g.write("[(")
		if err := g.emitExpr(node.Index); err != nil {
			return err
		}
		g.write(").toInt()]")
		return nil
	case *ast.ArrayLit:
		g.write("listOf(")
		for i, elem := range node.Elements {
			if i > 0 {
				g.write(", ")
			}
			if err := g.emitExpr(elem); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case *ast.ArrayLiteral:
		g.write("listOf(")
		for i, elem := range node.Elements {
			if i > 0 {
				g.write(", ")
			}
			if err := g.emitExpr(elem); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case *ast.MapLiteral:
		g.write("mapOf(")
		for i, entry := range node.Entries {
			if i > 0 {
				g.write(", ")
			}
			if err := g.emitExpr(entry.Key); err != nil {
				return err
			}
			g.write(" to ")
			if err := g.emitExpr(entry.Value); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case *ast.StructLit:
		g.write(node.TypeName + "(")
		for i, f := range node.Fields {
			if i > 0 {
				g.write(", ")
			}
			g.write(f.Name + " = ")
			if err := g.emitExpr(f.Value); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case *ast.CallExpr:
		if node.Callee == "println" {
			g.write("println(")
			for i, arg := range node.Args {
				if i > 0 {
					g.write(` + " " + `)
				}
				if i == 0 && len(node.Args) > 1 {
					g.write(`"" + `)
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write(")")
		} else {
			g.write(node.Callee + "(")
			for i, arg := range node.Args {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write(")")
		}
		return nil
	default:
		g.write(fmt.Sprintf("/* %s */", n.Kind()))
		return nil
	}
}

func (g *androidGen) write(s string)   { g.buf.WriteString(s) }
func (g *androidGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *androidGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func getAndroidRootBuildGradle() string {
	return `buildscript {
    repositories {
        google()
        mavenCentral()
    }
    dependencies {
        classpath 'com.android.tools.build:gradle:8.1.0'
        classpath 'org.jetbrains.kotlin:kotlin-gradle-plugin:1.8.20'
    }
}
allprojects {
    repositories {
        google()
        mavenCentral()
    }
}
`
}

func getAndroidSettingsGradle() string {
	return `rootProject.name = "XqlApp"
include ':app'
`
}

// getAndroidGradleProperties is required, not optional: app/build.gradle
// depends on androidx.core and androidx.appcompat, and AGP refuses to resolve
// AndroidX artifacts unless android.useAndroidX is set. Without this file the
// build fails at checkDebugAarMetadata.
func getAndroidGradleProperties() string {
	return `android.useAndroidX=true
org.gradle.jvmargs=-Xmx2048m
`
}

func getAndroidAppBuildGradle() string {
	return `plugins {
    id 'com.android.application'
    id 'org.jetbrains.kotlin.android'
}
android {
    namespace 'com.xql.app'
    compileSdk 33
    defaultConfig {
        applicationId "com.xql.app"
        minSdk 21
        targetSdk 33
        versionCode 1
        versionName "1.0"
    }
    buildTypes {
        release {
            minifyEnabled false
        }
    }
}
dependencies {
    implementation 'androidx.core:core-ktx:1.10.1'
    implementation 'androidx.appcompat:appcompat:1.6.1'
}
`
}

func getAndroidManifest() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application
        android:allowBackup="true"
        android:icon="@mipmap/ic_launcher"
        android:label="@string/app_name"
        android:theme="@style/Theme.AppCompat.Light.NoActionBar">
        <activity
            android:name=".MainActivity"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`
}

func getAndroidLayoutXml() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<ScrollView xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="match_parent"
    android:fillViewport="true"
    android:padding="16dp">
    <LinearLayout
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:orientation="vertical">
        <TextView
            android:id="@+id/tvOutput"
            android:layout_width="match_parent"
            android:layout_height="wrap_content"
            android:textSize="16sp"
            android:textColor="#000000" />
    </LinearLayout>
</ScrollView>
`
}

func getAndroidStringsXml() string {
	return `<resources>
    <string name="app_name">XqlApp</string>
</resources>
`
}
