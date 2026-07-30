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
		"build.gradle":                            []byte(getAndroidRootBuildGradle()),
		"settings.gradle":                         []byte(getAndroidSettingsGradle()),
		"app/build.gradle":                        []byte(getAndroidAppBuildGradle()),
		"app/src/main/AndroidManifest.xml":        []byte(getAndroidManifest()),
		"app/src/main/res/layout/activity_main.xml": []byte(getAndroidLayoutXml()),
		"app/src/main/res/values/strings.xml":      []byte(getAndroidStringsXml()),
		"app/src/main/java/com/xql/app/MainActivity.kt": ktCode,
	}

	return &ProjectOutput{
		MainCode: ktCode,
		Files:    files,
	}, nil
}

// GenerateAndroidKotlin produces the MainActivity.kt logic for Android target.
func GenerateAndroidKotlin(root ast.Node) ([]byte, error) {
	g := &androidGen{buf: &strings.Builder{}}
	return g.generate(root)
}

type androidGen struct {
	buf    *strings.Builder
	indent int
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

	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		for _, stmt := range fd.Body {
			if err := g.emitNode(stmt); err != nil {
				return nil, err
			}
		}
	}

	g.indent--
	g.writeln("}")

	// Emit non-main helper functions
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

func (g *androidGen) emitNode(n ast.Node) error {
	switch node := n.(type) {
	case *ast.VarDecl:
		return g.emitVarDecl(node)
	case *ast.ExprStmt:
		return g.emitExprStmt(node)
	case *ast.IfStmt:
		return g.emitIf(node)
	case *ast.ReturnStmt:
		return g.emitReturn(node)
	default:
		// Fallback to basic Kotlin expr / stmt emission
		return g.emitExprStmt(&ast.ExprStmt{Expr: node})
	}
}

func (g *androidGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	g.write(fmt.Sprintf("var %s", vd.Name))
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
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

func (g *androidGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.writeIndent()
	g.write(fmt.Sprintf("private fun %s(", fd.Name))
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(fmt.Sprintf("%s: Any", p.Name))
	}
	g.writeln(") {")
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

func (g *androidGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		g.write(fmt.Sprintf("%v", node.Value))
		return nil
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.CallExpr:
		if node.Callee == "println" {
			g.write("println(")
		} else {
			g.write(node.Callee + "(")
		}
		for i, arg := range node.Args {
			if i > 0 {
				g.write(", ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
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
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="match_parent"
    android:orientation="vertical"
    android:padding="16dp">
    <TextView
        android:id="@+id/tvOutput"
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:textSize="16sp"
        android:textColor="#000000" />
</LinearLayout>
`
}

func getAndroidStringsXml() string {
	return `resources>
    <string name="app_name">XqlApp</string>
</resources>
`
}
