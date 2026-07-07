{
  "kind": "Program",
  "declarations": [
    {
      "kind": "ImportDecl",
      "path": "./models.xql",
      "as": "models"
    },
    {
      "kind": "ImportDecl",
      "path": "./service.xql",
      "as": "service"
    },
    {
      "kind": "FunctionDecl",
      "name": "main",
      "params": [],
      "returnType": {"kind": "Void"},
      "effects": ["network", "io"],
      "grant": ["network:*", "io"],
      "body": [
        {
          "kind": "VarDecl",
          "name": "config",
          "type": {"kind": "models.Config"},
          "value": {
            "kind": "StructLit",
            "typeName": "models.Config",
            "fields": [
              {"name": "path", "value": {"kind": "Literal", "valueType": "String", "value": "./config.json"}},
              {"name": "max_retries", "value": {"kind": "Literal", "valueType": "Int", "value": 3}}
            ]
          }
        },
        {
          "kind": "VarDecl",
          "name": "res",
          "type": {
            "kind": "Result",
            "okType": {"kind": "Array", "elem": {"kind": "models.User"}},
            "errType": {"kind": "String"}
          },
          "value": {
            "kind": "CallExpr",
            "callee": "service.fetchUsers",
            "args": [
              {"kind": "Ident", "name": "config"}
            ]
          }
        },
        {
          "kind": "IfStmt",
          "condition": {
            "kind": "MemberExpr",
            "object": {"kind": "Ident", "name": "res"},
            "field": "isOk"
          },
          "then": [
            {
              "kind": "VarDecl",
              "name": "users",
              "type": {"kind": "Array", "elem": {"kind": "models.User"}},
              "value": {
                "kind": "CallExpr",
                "callee": "res.unwrap",
                "args": []
              }
            },
            {
              "kind": "ForStmt",
              "form": "each",
              "var": "user",
              "iterable": {"kind": "Ident", "name": "users"},
              "body": [
                {
                  "kind": "ExprStmt",
                  "expr": {
                    "kind": "CallExpr",
                    "callee": "println",
                    "args": [
                      {
                        "kind": "MemberExpr",
                        "object": {"kind": "Ident", "name": "user"},
                        "field": "name"
                      }
                    ]
                  }
                }
              ]
            }
          ],
          "else": [
            {
              "kind": "ExprStmt",
              "expr": {
                "kind": "CallExpr",
                "callee": "println",
                "args": [
                  {
                    "kind": "CallExpr",
                    "callee": "res.unwrapErr",
                    "args": []
                  }
                ]
              }
            }
          ]
        }
      ]
    }
  ]
}
