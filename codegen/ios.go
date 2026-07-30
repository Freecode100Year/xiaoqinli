package codegen

import (
	"xiaoqinli/ast"
)

// GenerateIOSProject produces a complete Swift Package Manager / iOS Swift project structure.
func GenerateIOSProject(root ast.Node) (*ProjectOutput, error) {
	swiftCode, err := GenerateSwift(root)
	if err != nil {
		return nil, err
	}

	appSwift := getIOSAppSwiftCode()
	pkgSwift := getIOSPackageSwiftCode()
	readme := getIOSReadmeCode()

	files := map[string][]byte{
		"Package.swift":             []byte(pkgSwift),
		"Sources/XqlApp/main.swift": swiftCode,
		"Sources/XqlApp/App.swift":  []byte(appSwift),
		"README.md":                 []byte(readme),
	}

	return &ProjectOutput{
		MainCode: swiftCode,
		Files:    files,
	}, nil
}

func getIOSPackageSwiftCode() string {
	return `// swift-tools-version:5.8
import PackageDescription

let package = Package(
    name: "XqlApp",
    platforms: [
        .iOS(.v14),
        .macOS(.v11)
    ],
    products: [
        .executable(name: "XqlApp", targets: ["XqlApp"])
    ],
    targets: [
        .executableTarget(
            name: "XqlApp",
            dependencies: []
        )
    ]
)
`
}

func getIOSAppSwiftCode() string {
	return `import Foundation

public struct XqlAppRunner {
    public static func run() {
        print("[XQL iOS] App initialized.")
    }
}
`
}

func getIOSReadmeCode() string {
	return `# XqlApp - iOS / Swift Package Manager Project

Generated automatically by Xiaoqinli (xql) AST Transpiler.

## How to Build & Run
- **Command Line**: Run ` + "`" + `swift build` + "`" + ` or ` + "`" + `swift run` + "`" + `
- **Xcode**: Double click ` + "`" + `Package.swift` + "`" + ` to open in Xcode, choose an iOS Simulator or Device, and press Command+R to build & run.
`
}
