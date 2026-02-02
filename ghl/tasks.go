package ghl

import "fmt"

type Task struct {
	Title      string `json:"title"`
	Body       string `json:"body"`
	DueDate    string `json:"dueDate"`
	Completed  bool   `json:"completed"`
	AssignedTo string `json:"assignedTo"`
}

func (req *GoHighLevelRequest) AddTask(ContactID string, task Task) error {
	endpoint := fmt.Sprintf("/contacts/%s/tasks", ContactID)
	var result map[string]any
	if err := req.POST(endpoint, task, &result); err != nil {
		return err
	}
	return nil
}
