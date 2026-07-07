{
  "kind": "Program",
  "declarations": [
    {
      "kind": "ImportDecl",
      "path": "./models.xql",
      "as": "models"
    },
    {
      "kind": "FunctionDecl",
      "name": "fetchUsers",
      "params": [
        {"name": "config", "type": {"kind": "models.Config"}}
      ],
      "returnType": {
        "kind": "Result",
        "okType": {"kind": "Array", "elem": {"kind": "models.User"}},
        "errType": {"kind": "String"}
      },
      "effects": ["network"],
      "grant": ["network:read"],
      "body": [
        {
          "kind": "VarDecl",
          "name": "users",
          "type": {"kind": "Array", "elem": {"kind": "models.User"}},
          "value": {
            "kind": "ArrayLiteral",
            "elemType": {"kind": "models.User"},
            "elements": [
              {
                "kind": "StructLit",
                "typeName": "models.User",
                "fields": [
                  {"name": "id", "value": {"kind": "Literal", "valueType": "Int", "value": 1}},
                  {"name": "name", "value": {"kind": "Literal", "valueType": "String", "value": "Alice"}}
                ]
              },
              {
                "kind": "StructLit",
                "typeName": "models.User",
                "fields": [
                  {"name": "id", "value": {"kind": "Literal", "valueType": "Int", "value": 2}},
                  {"name": "name", "value": {"kind": "Literal", "valueType": "String", "value": "Bob"}}
                ]
              }
            ]
          }
        },
        {
          "kind": "IfStmt",
          "condition": {
            "kind": "BinaryExpr",
            "op": "<",
            "left": {
              "kind": "MemberExpr",
              "object": {"kind": "Ident", "name": "config"},
              "field": "max_retries"
            },
            "right": {"kind": "Literal", "valueType": "Int", "value": 0}
          },
          "then": [
            {
              "kind": "ReturnStmt",
              "value": {
                "kind": "CallExpr",
                "callee": "Result.err",
                "args": [
                  {"kind": "Literal", "valueType": "String", "value": "invalid retries"}
                ]
              }
            }
          ],
          "else": [
            {
              "kind": "ReturnStmt",
              "value": {
                "kind": "CallExpr",
                "callee": "Result.ok",
                "args": [
                  {"kind": "Ident", "name": "users"}
                ]
              }
            }
          ]
        }
      ]
    }
  ]
}
