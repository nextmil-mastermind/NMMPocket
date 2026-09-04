# RSVP

PocketBase serves member RSVP pages and sends invite emails. Members open a slug URL (with a token from email, or by logging in). Staff create events in the `rsvp` collection and trigger send from `POST /rsvp/{slug}/send`.

## Flow

1. Create an RSVP in the dashboard (`title`, unique `slug`, optional filters, `email_template`).
2. New records default to **open** and **invite active members only**. Uncheck those after save if you need the opposite.
3. Send emails: `POST /rsvp/{slug}/send` as an authenticated `users` record.
4. Invited members click `{appurl}/rsvp/{slug}?token=...` or visit `{appurl}/rsvp/{slug}` and log in.
5. Token links set an HttpOnly cookie and redirect to `/rsvp/{slug}` so the token is not left in the address bar.
6. Invited members submit yes / no / maybe. Others see the not-invited page.

## Collections

### `rsvp` (event)

| Field | Purpose |
|---|---|
| `slug` | Required unique URL name (`q4-workshop`). Pattern: lowercase letters, numbers, hyphens. |
| `title` | Display name (falls back to `name`, then `slug`). |
| `members` | Optional explicit invite list. Empty = no extra ID filter. |
| `groups` | Optional group filter (values come from `members.group`). Empty = any group. |
| `invite_active_only` | If true: `expiration > now` or `group = founder`. |
| `email_template` | Relation to `email_templates`. Required to send. |
| `sent_at` | Set automatically after a successful send. |
| `open` | If false, invited members can view but not submit. |
| `not_invited_message` | Optional HTML shown on the not-invited page. |

### `rsvp_responses`

Admin-only API (custom routes write these). One row per invited member (`rsvp` + `member` unique).

| Field | Purpose |
|---|---|
| `rsvp` | Event |
| `member` | Member |
| `status` | `yes`, `no`, `maybe`, or empty (invited, not yet responded) |
| `guests` | Extra guests (integer, default 0) |
| `note` | Optional text |

Send upserts a pending row for each invited member so the dashboard can show invited vs responded.

## Who is invited

Filters **intersect**. A member must pass every filter that is set.

- Always exclude `info@nextmilmastermind.com`
- If `invite_active_only`: future `expiration` or `group = founder`
- If `groups` is set: member `group` must be in the list
- If `members` is set: member id must be in the list

Empty `groups` and empty `members` means all members who pass the other rules.

## Member URLs

Base URL is `appurl` (falls back to `https://pocket.nextmil.org`).

| URL | Behavior |
|---|---|
| `GET /rsvp/{slug}` | Login if anonymous. Form if invited. Not-invited page otherwise. |
| `GET /rsvp/{slug}?token=...` | Validate member auth token, set cookie, redirect to `/rsvp/{slug}`. |
| `POST /rsvp/{slug}/login` | Email/username + password against `members`. |
| `POST /rsvp/{slug}` | Save `status`, `guests`, `note` (invited + open only). |

Closed events stay on the form with a notice; they do not use the not-invited page.

## Sending emails

Authenticated `users` only (same gate as `/user_token`):

```http
POST /rsvp/{slug}/send
Authorization: <PocketBase users auth token>
```

Response:

```json
{
  "invited": 42,
  "emailed": 40,
  "errors": ["someone@example.com: ..."]
}
```

Re-sends are allowed. Existing response rows are kept; new tokens are generated.

### Email template params

`email_templates` HTML/subject can use Brevo `{{params.*}}`:

- `{{params.first_name}}`
- `{{params.last_name}}`
- `{{params.rsvp_url}}` — tokenized link
- `{{params.title}}`
- `{{params.slug}}`

Include a button or link to `{{params.rsvp_url}}`. Members without the email can still use `/rsvp/{slug}` and log in.

## Checklist

1. Run migrations so `rsvp` fields and `rsvp_responses` exist.
2. Create or pick an `email_templates` record with `{{params.rsvp_url}}`.
3. Create the RSVP with a unique slug and attach that template.
4. Set `groups` and/or `members` if the event is not for everyone eligible.
5. `POST /rsvp/{slug}/send`.
6. Confirm `sent_at` and `rsvp_responses` rows in the dashboard.
