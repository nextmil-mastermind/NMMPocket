package emailsender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

var emailBase *template.Template

// EmailSender sends an email using the Brevo API.
func EmailSender(to []Recipient, subject, message string, attachment *string, ticketurl bool) error {
	// Render email content.

	dataForTemplate := map[string]string{
		"subject": subject,
		"message": message,
	}
	var emailBuffer bytes.Buffer
	if err := emailBase.Execute(&emailBuffer, dataForTemplate); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}
	htmlContent := emailBuffer.String()

	// Build messageVersions.
	var messageVersions []MessageVersion
	for _, r := range to {
		params := map[string]any{
			"name":  r.Name,
			"email": r.Email,
		}
		if ticketurl {
			params["ticket"] = r.Ticket
		}
		mv := MessageVersion{
			To:     []Contact{{Name: r.Name, Email: r.Email}},
			Params: params,
		}
		messageVersions = append(messageVersions, mv)
	}

	// Build the complete payload.
	payload := EmailData{
		Sender: Contact{
			Name:  os.Getenv("BREVO_SENDER_NAME"),
			Email: os.Getenv("BREVO_SENDER_EMAIL"),
		},
		ReplyTo: Contact{
			Name:  "Next Million Mastermind",
			Email: "info@nextmilmastermind.com",
		},
		Subject:         subject,
		HtmlContent:     htmlContent,
		MessageVersions: messageVersions,
		Attachment:      attachment,
	}

	// Marshal payload to JSON.
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	// Prepare HTTP request.
	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("accept", "application/json")
	req.Header.Set("api-key", os.Getenv("BREVO_API_KEY"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// If response is not OK, read the error body.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// SendOrderEmail sends order confirmation emails.
// It first sends an email to the order owner and then, if there are multiple tickets,
// sends individual attendee emails.
func SendOrderEmail(data OrderData) error {
	// Build ticket URL for the order owner.
	ticketURL := os.Getenv("order_url") + "/" + data.OrderReference + "?view"

	// Build the order message.
	orderMsg, _ := orderMessage(map[string]any{
		"first_name": data.FirstName,
		"event":      data.Title,
		"location":   data.Venue,
		"date":       data.StartTime.Format("01/02/2006 03:04 PM"),
		"ticket_no":  len(data.Tickets),
		"address":    data.Address,
		"ticket":     ticketURL,
	})

	// Initial recipient list (the order owner).
	to := []Recipient{
		{
			Name:   data.FirstName + " " + data.LastName,
			Email:  data.Email,
			Ticket: ticketURL,
		},
	}

	// Send the order email.
	if err := EmailSender(to, "Your tickets for "+data.Title, orderMsg, nil, true); err != nil {
		log.Println(err)
		return err
	}

	// If more than one ticket, send attendee emails.
	if len(data.Tickets) > 1 {
		var attendeeRecipients []Recipient
		for _, ticket := range data.Tickets {
			if !ticket["main"].(bool) {
				parts := strings.Split(ticket["Reference"].(string), "-")
				if len(parts) >= 2 {
					attendeeTicketURL := os.Getenv("order_url") + "/" + parts[0] + "/" + parts[1] + "?view"
					attendeeRecipients = append(attendeeRecipients, Recipient{
						Name:   ticket["FirstName"].(string) + " " + ticket["LastName"].(string),
						Email:  ticket["Email"].(string),
						Ticket: attendeeTicketURL,
					})
				}
			}
		}

		// Build the attendee ticket message.
		addr := ""
		if data.Address != nil {
			addr = *data.Address
		}
		attendeeMsg, _ := attendeeMessage(map[string]any{
			"event":    data.Title,
			"location": data.Venue,
			"date":     data.StartTime.Format("01/02/2006 03:04 PM"),
			"address":  addr,
			"ticket":   ticketURL,
		})

		if err := EmailSender(attendeeRecipients, "Your ticket for "+data.Title, attendeeMsg, nil, true); err != nil {
			log.Println(err)
			return err
		}

		log.Println("Emails sent for order " + data.OrderReference)
	}

	return nil
}
func LoadEmailTemplate() (*template.Template, error) {
	emailFile, err := os.Open("emailbase.html")
	if err != nil {
		log.Fatal(err)
	}
	defer emailFile.Close()
	emailString, err := io.ReadAll(emailFile)
	if err != nil {
		log.Fatal(err)
	}
	tmpl, err := template.New("email").Parse(string(emailString))
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	return tmpl, nil
}

// orderMessage builds an order confirmation email message.
func orderMessage(data map[string]any) (string, error) {
	// Extract required values.
	firstName, ok := data["first_name"].(string)
	if !ok {
		return "", fmt.Errorf("first_name not found or not a string")
	}
	event, ok := data["event"].(string)
	if !ok {
		return "", fmt.Errorf("event not found or not a string")
	}
	location, ok := data["location"].(string)
	if !ok {
		return "", fmt.Errorf("location not found or not a string")
	}
	date, ok := data["date"].(string)
	if !ok {
		return "", fmt.Errorf("date not found or not a string")
	}
	ticketNo, ok := data["ticket_no"]
	if !ok {
		return "", fmt.Errorf("ticket_no not found")
	}

	// Optionally add the address.
	addressStr := ""
	if addr, exists := data["address"]; exists {
		if addrStr, ok := addr.(string); ok && addrStr != "" {
			addressStr = fmt.Sprintf("<li><strong>Address: </strong>%s</li>", addrStr)
		}
	}

	// Build the message.
	msg := fmt.Sprintf(`<div>Hi %s,</div>
<div>&nbsp;</div>
<div>Thanks for registering for %s.</div>
<div>Your tickets are attached to this email and individual tickets have been sent to all applicable guests.&nbsp;</div>
<div>&nbsp;</div>
<div><strong><em>Event Details:</em></strong>&nbsp;<br /><strong>%s</strong></div>
<ul>
<li><strong>Where? </strong>%s</li>
%s
<li><strong>When? </strong>%s</li>
<li><strong>Number of tickets: </strong>%v</li>
<li><strong>Print Your Ticket(s): </strong><a href="{{params.ticket}}">Click Here!</a></li>
</ul>
<div>&nbsp;</div>
<div>Take care,&nbsp;</div>
<div>&nbsp;</div>
<div>Next Million Mastermind</div>`,
		firstName, event, event, location, addressStr, date, ticketNo)

	return msg, nil
}

// attendeeMessage builds an attendee email message.
func attendeeMessage(data map[string]any) (string, error) {
	event, ok := data["event"].(string)
	if !ok {
		return "", fmt.Errorf("event not found or not a string")
	}
	location, ok := data["location"].(string)
	if !ok {
		return "", fmt.Errorf("location not found or not a string")
	}
	date, ok := data["date"].(string)
	if !ok {
		return "", fmt.Errorf("date not found or not a string")
	}

	addressStr := ""
	if addr, exists := data["address"]; exists {
		if addrStr, ok := addr.(string); ok && addrStr != "" {
			addressStr = fmt.Sprintf("<li><strong>Address: </strong>%s</li>", addrStr)
		}
	}

	msg := fmt.Sprintf(`<div>Hi {{params.name}},</div>
<div>&nbsp;</div>
<div>Thanks for registering for %s.</div>
<div>Your ticket is available using the link below:</div>
<div>&nbsp;</div>
<div><strong><em>Event Details:</em></strong>&nbsp;<br /><strong>%s</strong></div>
<ul>
<li><strong>Where? </strong>%s</li>
<li><strong>When? </strong>%s</li>
%s
<li><strong>Print Your Ticket: </strong><a href="{{params.ticket}}">Click Here!</a></li>
</ul>
<div>&nbsp;</div>
<div>Take care,&nbsp;</div>
<div>&nbsp;</div>
<div>Next Million Mastermind</div>`,
		event, event, location, date, addressStr)

	return msg, nil
}
