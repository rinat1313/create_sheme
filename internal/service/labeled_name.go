package service

import "log/slog"

type LabeledName struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func toLabeledNames(names []string, labels map[string]string) []LabeledName {
	if names == nil {
		return []LabeledName{}
	}
	out := make([]LabeledName, 0, len(names))
	for _, name := range names {
		item := LabeledName{Name: name}
		if labels != nil {
			item.Description = labels[name]
		}
		out = append(out, item)
	}
	slog.Debug("toLabeledNames done", "count", len(out), "labels", len(labels))
	return out
}
