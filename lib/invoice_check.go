package lib

import (
	"log"
	"net/mail"
	"reflect"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/tools/mailer"
	"github.com/pocketbase/pocketbase/tools/types"
)

type Invoice struct {
	ID            string                `db:"id"`
	Name          string                `db:"name"`
	Email         string                `db:"email"`
	InvoiceName   string                `db:"invoicename"`
	Description   string                `db:"description"`
	Amount        float64               `db:"amount"`
	DueDate       types.DateTime        `db:"duedate"`
	Session       string                `db:"session"`
	Paid          bool                  `db:"paid"`
	Reminders     bool                  `db:"reminders"`
	CC            *types.JSONArray[any] `db:"cc"`
	InvoiceType   string                `db:"type"`
	SessionURL    string                `db:"sessionurl"`
	DaysRemaining int                   `db:"days_remaining"`
}

var InvoiceKeys = []string{
	"ID",
	"Name",
	"Email",
	"InvoiceName",
	"Description",
	"Amount",
}

type EmailTemplate struct {
	ID      string `db:"id"`
	Name    string `db:"name"`
	Subject string `db:"subject"`
	Body    string `db:"html"`
	Days    int    `db:"days"`
}

func CheckInvoice(app *pocketbase.PocketBase) {
	var res []Invoice
	err := app.DB().Select(
		"*, CAST(julianday(date(duedate)) - julianday(date('now')) as INTEGER) as days_remaining",
	).
		From("invoices").
		Where(dbx.NewExp("paid = {:paid}", dbx.Params{"paid": false})).
		AndWhere(dbx.NewExp("date(duedate) IN (date('now'), date('now', '+1 day'), date('now', '+5 day'), date('now', '+20 day'))", nil)).
		All(&res)
	if err != nil {
		log.Default().Println(err)
		return
	}
	if len(res) == 0 {
		return
	}
	templates := getEmailTemplates(app.DB())
	for _, invoice := range res {
		if invoice.DaysRemaining != 0 && invoice.Reminders {
			message := sendReminderEmail(invoice, templates[invoice.DaysRemaining])
			log.Default().Println(message)
			err := app.NewMailClient().Send(message)
			if err != nil {
				log.Default().Println(err)
			}
		} else if invoice.InvoiceType == "auto" {
			// Auto pay invoice
			_, err := createStripeCharge(invoice, app)
			if err != nil {
				log.Default().Println(err)
			}
		} else {
			log.Default().Println("No email to send")
		}
	}
}

func sendReminderEmail(invoice Invoice, template EmailTemplate) *mailer.Message {
	messageText := paramsCleanUp(template.Body, invoice)
	if invoice.InvoiceType == "auto" {
		messageText = strings.Replace(messageText, "{{params.is_auto_pay}}", "<bold>Note:</bold> This invoice will be automatically billed to your card on the due date.<br>", -1)
	} else {
		messageText = strings.Replace(messageText, "{{params.is_auto_pay}}", "", -1)
	}
	message := &mailer.Message{
		From: mail.Address{
			Address: "info@nextmilmastermind.com",
			Name:    "Next Mil Mastermind",
		},
		To:      []mail.Address{{Address: invoice.Email, Name: invoice.Name}},
		Subject: paramsCleanUp(template.Subject, invoice),
		HTML:    messageText,
	}
	return message
}

func getEmailTemplates(db dbx.Builder) map[int]EmailTemplate {
	var res []EmailTemplate
	db.Select("*").From("email_basic").Where(dbx.NewExp("is_invoice = true")).All(&res)
	var templates = make(map[int]EmailTemplate)
	for _, template := range res {
		templates[template.Days] = template
	}
	return templates
}

func paramsCleanUp(Text string, invoice Invoice) string {
	v := reflect.ValueOf(invoice)
	for _, key := range InvoiceKeys {
		field := v.FieldByName(key)
		if field.IsValid() && field.Kind() == reflect.String {
			Text = strings.Replace(Text, "{{params."+key+"}}", field.String(), -1)
		}
	}
	first_name := strings.Split(invoice.Name, " ")[0]
	Text = strings.Replace(Text, "{{params.first_name}}", first_name, -1)
	Text = strings.Replace(Text, "{{params.DueDate}}", convertTimeToString(invoice.DueDate), -1)
	return Text
}

func convertTimeToString(t types.DateTime) string {
	return t.Time().Format("Jan 2, 2006")
}
