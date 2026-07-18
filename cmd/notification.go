package cmd

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

var notificationCmd = &cobra.Command{
	Use:     "notification",
	Aliases: []string{"notifications", "notif"},
	Short:   "Router notifications / alerts",
	RunE: func(cmd *cobra.Command, args []string) error {
		return notificationListCmd.RunE(cmd, args)
	},
}

var notificationListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show notification config (alert rules + contacts + enable) or pending entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		raw, err := c().Notifications()
		if err != nil {
			return err
		}
		if emit(raw) {
			return nil
		}
		// The 25.x Bbox firmware returns notification CONFIG at this endpoint:
		// {enable, alerts[], contacts[], events[]}. Detect and render that shape
		// specifically; fall back to the historical instance-list shape.
		if cfg, ok := notificationConfigShape(raw); ok {
			printNotificationConfig(cfg)
			return nil
		}
		entries := notificationEntries(raw)
		if len(entries) == 0 {
			fmt.Println("(no notifications)")
			return nil
		}
		for _, e := range entries {
			fmt.Println(formatNotification(e))
		}
		return nil
	},
}

// notificationConfigShape returns the inner config map if the payload looks
// like {enable, alerts, contacts[, events]}. Handles all three observed
// wrappers: bare map, [{...}] envelope, and [{notification: {...}}].
func notificationConfigShape(raw any) (map[string]any, bool) {
	isConfig := func(m map[string]any) bool {
		_, a := m["alerts"]
		_, c := m["contacts"]
		_, e := m["enable"]
		return a && c && e
	}
	tryMap := func(v any) (map[string]any, bool) {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		if isConfig(m) {
			return m, true
		}
		if inner, ok := m["notification"].(map[string]any); ok && isConfig(inner) {
			return inner, true
		}
		return nil, false
	}
	if m, ok := tryMap(raw); ok {
		return m, true
	}
	if lst, ok := raw.([]any); ok && len(lst) > 0 {
		return tryMap(lst[0])
	}
	return nil, false
}

func printNotificationConfig(m map[string]any) {
	fmt.Println("Notification service")
	printKV([][2]any{{"enabled", fmtBool(m["enable"])}})

	if alerts, ok := m["alerts"].([]any); ok && len(alerts) > 0 {
		fmt.Printf("\nAlert rules (%d)\n", len(alerts))
		for _, a := range alerts {
			am, _ := a.(map[string]any)
			action := getMap(am, "action")
			channel := toStr(action["type"])
			if channel == "" {
				channel = "-"
			}
			fmt.Printf("  #%v  %-10s  enable=%s  action=%s  events=%s\n",
				dash(am["id"]), dash(am["name"]), fmtBool(am["enable"]),
				channel, dash(am["events"]))
		}
	}

	if contacts, ok := m["contacts"].([]any); ok && len(contacts) > 0 {
		fmt.Printf("\nContacts (%d)\n", len(contacts))
		for _, c := range contacts {
			cm, _ := c.(map[string]any)
			mail := toStr(cm["mail"])
			if mail == "" {
				mail = "(not set)"
			}
			fmt.Printf("  #%v  %s  %s  enable=%s\n",
				dash(cm["id"]), dash(cm["name"]), mail, fmtBool(cm["enable"]))
		}
	}

	if events, ok := m["events"].([]any); ok && len(events) > 0 {
		fmt.Printf("\nAvailable event types: %d (use 'bbox notification events' to list, --json for full catalog)\n", len(events))
	}
}

var notificationClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all notifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().NotificationsClear(); err == nil {
			fmt.Println("OK: notifications cleared")
			return nil
		} else {
			// Fall back to per-id deletion.
			raw, gerr := c().Notifications()
			if gerr != nil {
				return err
			}
			entries := notificationEntries(raw)
			if len(entries) == 0 {
				return err
			}
			n := 0
			for _, e := range entries {
				id := toIntAny(e["id"])
				if id == 0 {
					continue
				}
				if delErr := c().NotificationDel(id); delErr == nil {
					n++
				}
			}
			if n == 0 {
				return err
			}
			fmt.Printf("OK: deleted %d notification(s)\n", n)
			return nil
		}
	},
}

// notificationEntries normalises the parsed body into a flat []map. The Bbox
// firmware has returned three shapes for /api/v1/notification in the wild:
//   - a plain array of entries
//   - `[{"notification": [...]}]`
//   - `[{"notification": {...single...}}]`
func notificationEntries(raw any) []map[string]any {
	var out []map[string]any
	pushList := func(l []any) {
		for _, e := range l {
			if m, ok := e.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	switch v := raw.(type) {
	case []any:
		// Either a flat array of entries, or the loose-dict `[{key: ...}]` shape.
		looksLikeEntries := false
		for _, e := range v {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			if _, hasWrap := m["notification"]; !hasWrap {
				looksLikeEntries = true
				break
			}
		}
		if looksLikeEntries {
			pushList(v)
			return out
		}
		for _, e := range v {
			m, _ := e.(map[string]any)
			inner, ok := m["notification"]
			if !ok {
				continue
			}
			switch n := inner.(type) {
			case []any:
				pushList(n)
			case map[string]any:
				out = append(out, n)
			}
		}
	case map[string]any:
		out = append(out, v)
	}
	return out
}

// formatNotification pretty-prints a single entry. Falls back to a compact JSON
// dump if none of the expected fields are present.
func formatNotification(e map[string]any) string {
	date := toStr(e["date"])
	sev := toStr(e["severity"])
	if sev == "" {
		sev = toStr(e["type"])
	}
	title := toStr(e["title"])
	msg := toStr(e["message"])
	if date == "" && title == "" && msg == "" {
		b, _ := json.Marshal(e)
		return string(b)
	}
	body := title
	if msg != "" {
		if body != "" {
			body += ": " + msg
		} else {
			body = msg
		}
	}
	sevOut := sev
	if sevOut == "" {
		sevOut = "-"
	}
	dateOut := date
	if dateOut == "" {
		dateOut = "-"
	}
	return fmt.Sprintf("%s  %s  %s", dateOut, sevOut, body)
}

var notificationEventsCategory string

var notificationEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "List the router's notification event catalog",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		raw, err := c().Notifications()
		if err != nil {
			return err
		}
		cfg, ok := notificationConfigShape(raw)
		if !ok {
			return fmt.Errorf("notification endpoint returned unexpected shape (no 'events' array)")
		}
		events, _ := cfg["events"].([]any)
		filter := notificationEventsCategory
		if emit(events) {
			return nil
		}
		byCat := map[string][]map[string]any{}
		for _, e := range events {
			em, _ := e.(map[string]any)
			cat := toStr(em["category"])
			if filter != "" && cat != filter {
				continue
			}
			byCat[cat] = append(byCat[cat], em)
		}
		cats := make([]string, 0, len(byCat))
		for k := range byCat {
			cats = append(cats, k)
		}
		sort.Strings(cats)
		for _, cat := range cats {
			fmt.Printf("\n[%s]\n", cat)
			for _, em := range byCat[cat] {
				fmt.Printf("  %-40s %s\n", dash(em["name"]), dash(em["humanReadable"]))
			}
		}
		return nil
	},
}

func init() {
	notificationEventsCmd.Flags().StringVar(&notificationEventsCategory, "category", "", "filter by category (e.g. Internet, Système, Téléphone)")
	notificationCmd.AddCommand(notificationListCmd, notificationClearCmd, notificationEventsCmd)
	rootCmd.AddCommand(notificationCmd)
}
