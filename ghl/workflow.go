package ghl

import "time"

func (req *GoHighLevelRequest) AddContactToWorkflow(ContactID, WorkflowID string) error {
	now := time.Now()
	add5mins := now.Add(5 * time.Minute).Format(time.RFC3339)
	data := map[string]any{
		"eventStartTime": add5mins,
	}
	var result map[string]any
	endpoint := "/contacts/" + ContactID + "/workflow/" + WorkflowID
	err := req.POST(endpoint, data, &result)
	if err != nil {
		return err
	}
	return nil
}
