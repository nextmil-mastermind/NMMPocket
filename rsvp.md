# RSVP

PocketBase owns the full RSVP flow: event records, invite filtering, Brevo emails with magic-link tokens, member login, Accept/Decline, and response storage.

Public URL shape: `{appurl}/rsvp/{slug}`  
Example: `https://pocket.nextmil.org/rsvp/september-2026-3-day`

`appurl` comes from the environment. If it is empty, links fall back to `https://pocket.nextmil.org`.

---

## Architecture

```
Staff (users)                 Members
     |                            |
     |  dashboard: create rsvp    |
     |  POST /rsvp/{slug}/send    |
     v                            |
 Resolve invite list              |
 Upsert rsvp_response rows        |
 Brevo email with ?token=         |
     |                            v
     +------------------> GET /rsvp/{slug}?token=
                          cookie + redirect to /rsvp/{slug}
                          or login at /rsvp/{slug}
                          invited  -> form (message + Accept/Decline)
                          not listed -> not-invited page
                          POST /rsvp/{slug} -> rsvp_response
```

Code lives in `rsvp/`. HTML is in `rsvp/html/` (prod copies to `/pb/rsvphtml/`).

---

## Collections

### `rsvp` (event)

One record per event. Looked up by **`slug`**, never by record id.

PocketBase also has system fields: `id`, `created`, `updated`, `collectionId`, `collectionName`.

#### Fields the app uses

| Field | Type | Required | Default / notes |
|---|---|---|---|
| `title` | text | yes (if present) | Shown on every page. Fallback order: `title` → `name` → `slug`. |
| `slug` | text, unique | yes | URL name. Pattern `^[a-z0-9]+(?:-[a-z0-9]+)*$`. Example: `september-2026-3-day`. Unique index `idx_rsvp_slug`. |
| `message` | editor (HTML) | no | Rendered on the invited form above Accept/Decline. |
| `not_invited_message` | editor (HTML) | no | Rendered after login when the member is not invited. Empty uses a built-in default. |
| `members` | multi relation → `members` | no | If this list has anyone, only those members may RSVP / get email. Empty + `members_only` = every member (then `invite_active_only` can narrow it). |
| `members_only` | bool | no | If **true** and `members` is empty, any member may register. If **false** and `members` is empty, nobody is invited. |
| `groups` | multi select | no | Must match `members.group`. Values are copied from the `members.group` select when possible (at least `founder`). Empty = any group. |
| `invite_active_only` | bool | no | Forced **true** on create. If true, the invite set is reduced to active members: `expiration > now` **or** `group = founder`. |
| `expiration` | date | no | **Event** RSVP deadline. After this time invited members can view but not submit. Not the same as member membership expiration. |
| `open` | bool | no | Forced **true** on create. If false, form is view-only. |
| `allow_guests` | bool | no | If true, the form shows Additional guests. Off by default. |
| `email_template` | relation → `email_templates` | required to send | Subject + HTML for Brevo. |
| `sent_at` | date | no | Set automatically after a successful send. |

On create, a hook always sets `invite_active_only = true` and `open = true`. Uncheck them after the first save if you want the opposite.

#### Fields that exist on live records but are not used by this app yet

These can stay on the record; the current routes ignore them.

| Field | Seen on live data | Current behavior |
|---|---|---|
| `additional` | relation/list, often `[]` | Ignored. Not part of invite or send. |
| `additional_too` | bool | Ignored. |
| `conditional_message` | JSON (`additional`, `members`, `messages`) | Ignored. Everyone sees `message` or `not_invited_message`. |

#### Example record

```json
{
  "id": "9pb4qwm2qck5zzs",
  "title": "September 2026 3-Day",
  "slug": "september-2026-3-day",
  "members_only": true,
  "members": ["vrfyk60df47phvk", "35xwietvmv21d4s"],
  "groups": [],
  "invite_active_only": false,
  "expiration": "2026-09-11 22:00:00.000Z",
  "open": true,
  "allow_guests": false,
  "email_template": "",
  "sent_at": "",
  "message": "<h3>Hot Seat Time</h3><p>Click Accept or Decline below.</p>",
  "not_invited_message": "<p>You are not on the invite list.</p>"
}
```

With this shape: `members` is populated, so only those IDs can Accept/Decline. Clear `members` and leave `members_only` true to let every member register (then turn on `invite_active_only` to keep only active ones). After `2026-09-11 22:00:00Z` the form stays visible for invited members but submit is disabled. Send will fail until `email_template` is set.

---

### `rsvp_response` (one row per member per event)

**Use this collection.** Do not use `rsvp_responses` (that extra collection is deleted by migration `1788551000_rsvp_response.go`).

API rules are locked for public CRUD. Only the custom `/rsvp/...` routes write rows.

The code detects field names at runtime (first match wins):

| Role | Accepted field names | Type |
|---|---|---|
| Event link | `rsvp`, then `event` | relation → `rsvp` |
| Member link | `member`, then `user` | relation → `members` |
| Decision | `status`, then `response`, then `attending`, then `accepted` | select **or** bool |
| Extra guests | `guests` | number, integer ≥ 0 (shown only when the event has `allow_guests`) |
| Note | `note` | text, max 2000 (optional; form hides if missing) |

If a decision field is missing, migration adds `status` as a select: `accept`, `decline`.  
If `guests` / `note` are missing, migration adds them.  
Unique index `idx_rsvp_response_unique` on `(rsvp|event, member|user)` when those relations exist.

#### How Accept/Decline is stored

The form always posts `status=accept` or `status=decline`. That is mapped onto whatever decision field exists:

| Decision field | Accept stored as | Decline stored as |
|---|---|---|
| bool (`attending` / `accepted`) | `true` | `false` |
| select containing `accept` / `yes` / `accepted` / `attending` | that value | — |
| select containing `decline` / `no` / `declined` | — | that value |
| select with other values | first value | second value (or first if only one) |

Pending rows created at send time have no decision yet (empty select / false bool). That is how you tell invited vs responded in the dashboard.

---

### `members` (auth collection)

Used for login, invite checks, and email recipients.

| Field | How RSVP uses it |
|---|---|
| `id` | Must match an ID in `rsvp.members` when that list is non-empty. |
| `email` | Login, Brevo `to`, token identity. |
| `username` | Alternate login if email lookup fails. |
| `password` | `POST /rsvp/{slug}/login`. |
| `first_name`, `last_name` | Email params and greeting. |
| `group` | `groups` filter; `founder` bypasses `invite_active_only`. |
| `expiration` | Membership end date. Used only when `invite_active_only` is true. |

Auth tokens are PocketBase member auth tokens (`NewAuthToken` / `FindAuthRecordByToken`).

---

### `email_templates`

Relation target of `rsvp.email_template`.

| Field | Use |
|---|---|
| `subject` | Brevo subject (may include `{{params.*}}`). |
| `html` | Brevo HTML body. |

Template params sent per recipient:

| Param | Value |
|---|---|
| `{{params.first_name}}` | Member first name |
| `{{params.last_name}}` | Member last name |
| `{{params.name}}` | Full name (also set by the mailer) |
| `{{params.email}}` | Member email |
| `{{params.rsvp_url}}` | `{appurl}/rsvp/{slug}?token={authToken}` |
| `{{params.title}}` | Event title |
| `{{params.slug}}` | Event slug |

Put a button/link on `{{params.rsvp_url}}`. Members can also open `/rsvp/{slug}` and log in without the email.

---

## Who is invited

Rules apply in order. Fail any one → not invited.

1. If `invite_active_only`: keep only active members (`expiration` in the future **or** `group = founder`).
2. If `groups` has values: member `group` must be in that list.
3. If `members` has anyone: only those IDs. If `members` is empty and `members_only` is true: any remaining member. If `members` is empty and `members_only` is false: nobody.

`invite_active_only` narrows whatever set step 3 produced (all members, or the explicit list).

`info@nextmilmastermind.com` is **not** excluded from RSVP. Zoom registration still skips that address.

Member IDs are collected from `GetStringSlice("members")` and from `ExpandedAll("members")`.

### Invite matrix

| `members_only` | `members` list | `invite_active_only` | Who can RSVP / get email |
|---|---|---|---|
| true | empty | false | Every member |
| true | empty | true | Every **active** member (or founder) |
| true | has IDs | false | Only those IDs |
| true | has IDs | true | Only those IDs who are active (or founder) |
| false | empty | * | Nobody |
| false | has IDs | true/false | Only those IDs (active filter if on) |

---

## When the form is closed

Invited members still see the form. Submit is disabled when:

- `open` is false, **or**
- `expiration` is set and `now >= expiration`

Not-invited members never see the form, even if the event is open.

---

## HTTP routes

All under `/rsvp`. Cookie middleware loads `e.Auth` from cookie `rsvp_auth` when the Authorization header is absent. Cookie: HttpOnly, `Path=/rsvp`, 14 days, Secure in TLS/prod, SameSite Lax.

| Method | Path | Who | Behavior |
|---|---|---|---|
| `POST` | `/rsvp/{slug}/send` | Authenticated **`users`** only | Resolve invitees, upsert pending `rsvp_response` rows, send Brevo, set `sent_at`. |
| `GET` | `/rsvp/{slug}?token=` | Public | Validate member auth token. Set cookie. **Redirect** to `/rsvp/{slug}` (token stripped from the URL). Invalid token → error page with login link. |
| `GET` | `/rsvp/{slug}` | Public | No auth → login. Invited member → form. Other member → not-invited page. |
| `POST` | `/rsvp/{slug}/login` | Public | Email/username + password against `members`. Sets cookie. Then same as GET. |
| `POST` | `/rsvp/{slug}` | Invited member (cookie) | Body: `status=accept\|decline`, optional `guests` (if `allow_guests`), `note`. Upserts `rsvp_response`. Confirmation page. |

Unknown slug → “RSVP not found”.  
Login failures use a generic “Invalid credentials.”

### Send response

```http
POST /rsvp/{slug}/send
Authorization: <PocketBase users token>
```

```json
{
  "invited": 11,
  "emailed": 11,
  "errors": ["someone@example.com: ..."]
}
```

Re-sends keep existing response rows and mint new tokens. `email_template` must be set or send returns an error.

---

## Pages

| File | When |
|---|---|
| `rsvp/html/login.html` | Anonymous GET, or failed/required login |
| `rsvp/html/form.html` | Invited member: `message` HTML + Accept/Decline (+ guests if `allow_guests`, + note if that field exists) |
| `rsvp/html/confirmation.html` | After a successful submit |
| `rsvp/html/not_invited.html` | Authenticated member who failed invite rules |
| `rsvp/html/error.html` | Missing event or bad/expired token |

---

## Environment

| Variable | Role |
|---|---|
| `appurl` | Public origin for email links (no trailing slash needed). |
| `is_prod` | `true` loads HTML from `/pb/rsvphtml/`; otherwise `rsvp/html/`. Cookie Secure when prod or TLS. |
| `SENDER_NAME`, `SENDER_EMAIL`, `REPLY_NAME`, `REPLY_EMAIL`, `BREVO_API_KEY` | Same Brevo path as Zoom/invoice mail. |

---

## Migrations

| File | What it does |
|---|---|
| `migrations/1788550000_rsvp.go` | Adds missing `rsvp` fields (`slug`, `members`, `groups`, `invite_active_only`, `email_template`, `sent_at`, `open`, `not_invited_message`, `allow_guests`). Backfills unique slugs on existing rows, then creates `idx_rsvp_slug`. |
| `migrations/1788551000_rsvp_response.go` | Deletes `rsvp_responses` if present. Ensures `rsvp_response` has event/member relations, a decision field, `guests`, `note`, and unique index. |
| `migrations/1788552000_rsvp_allow_guests.go` | Adds `allow_guests` on existing `rsvp` collections. |

---

## Operator checklist

1. Apply migrations (including slug backfill and `rsvp_response` patch).
2. Create/select an `email_templates` row that links to `{{params.rsvp_url}}`.
3. Create or edit the `rsvp` record:
   - unique `slug`
   - `title` and `message`
   - `not_invited_message`
   - `members_only` true + empty `members` = all members; add IDs to `members` to restrict
   - `invite_active_only` to keep only active members / founders
   - `expiration` deadline if needed
   - `allow_guests` if members should enter extra guests
   - attach `email_template`
4. To test the form as yourself: either be in `members`, or clear `members` with `members_only` on.
5. `POST /rsvp/{slug}/send` as a `users` auth token.
6. In the dashboard: `sent_at` set, one `rsvp_response` row per invited member.
7. Open `/rsvp/{slug}` as an invited member → Accept/Decline. As anyone else → not-invited page.

---

## Related files

- `rsvp/routes.go` — HTTP
- `rsvp/members.go` — invite + open/deadline
- `rsvp/send.go` — email
- `rsvp/response.go` — `rsvp_response` field mapping
- `rsvp/hooks.go` — create defaults
- `rsvp/html/` — pages
- `main.go` — `rsvp.RegisterHooks` + `rsvp.Routes`
- `Dockerfile` — copies HTML to `/pb/rsvphtml/`
