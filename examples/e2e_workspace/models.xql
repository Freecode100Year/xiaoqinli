{
  "kind": "Program",
  "declarations": [
    {
      "kind": "StructDecl",
      "name": "Config",
      "fields": [
        {"name": "path", "type": {"kind": "String"}},
        {"name": "max_retries", "type": {"kind": "Int"}}
      ]
    },
    {
      "kind": "StructDecl",
      "name": "User",
      "fields": [
        {"name": "id", "type": {"kind": "Int"}},
        {"name": "name", "type": {"kind": "String"}}
      ]
    }
  ]
}
