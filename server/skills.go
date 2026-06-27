package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"xiaoqinli/skills"
)

// SkillEntry describes one embedded skill document.
type SkillEntry struct {
	Name    string `json:"name"`
	Content string `json:"content,omitempty"`
}

// ListSkills returns metadata for all embedded skill documents.
func ListSkills() []SkillEntry {
	entries, err := fs.ReadDir(skills.FS, ".")
	if err != nil {
		return nil
	}
	out := make([]SkillEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		out = append(out, SkillEntry{Name: name})
	}
	return out
}

// GetSkill returns the content of a skill by name.
func GetSkill(name string) (string, bool) {
	data, err := fs.ReadFile(skills.FS, name+".md")
	if err != nil {
		return "", false
	}
	return string(data), true
}

// ---------------------------------------------------------------------------
// MCP prompts integration
// ---------------------------------------------------------------------------

func (s *MCPServer) handlePromptsList(req *jsonRPCRequest) jsonRPCResponse {
	list := ListSkills()
	prompts := make([]map[string]interface{}, 0, len(list))
	for _, sk := range list {
		prompts = append(prompts, map[string]interface{}{
			"name":        sk.Name,
			"description": "Xiaoqinli skill: " + sk.Name,
		})
	}
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"prompts": prompts},
	}
}

func (s *MCPServer) handlePromptsGet(req *jsonRPCRequest) jsonRPCResponse {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.Name == "" {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "missing required param 'name'"},
		}
	}

	content, ok := GetSkill(params.Name)
	if !ok {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "skill not found: " + params.Name},
		}
	}

	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"role": "user",
					"content": map[string]string{
						"type": "text",
						"text": content,
					},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// REST /skills/* endpoint
// ---------------------------------------------------------------------------

func handleSkillsREST(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/skills/")
	name = strings.TrimSuffix(name, "/")

	if name == "" {
		list := ListSkills()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"skills": list})
		return
	}

	content, ok := GetSkill(name)
	if !ok {
		http.Error(w, "skill not found: "+name, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write([]byte(content))
}
