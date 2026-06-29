package cli

import "github.com/charmbracelet/huh"

// This file holds thin wrappers around the most common single-field huh forms,
// so the many "ask for one thing" call sites stay one line instead of repeating
// the NewForm(NewGroup(...)).Run() boilerplate. Richer, multi-field or
// conditionally-built forms (e.g. the config wizard, `cert generate`) keep
// building huh directly - these helpers are only for the simple cases.

// promptInput asks for a single line of free text.
func promptInput(title, placeholder string, value *string) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Title(title).Placeholder(placeholder).Value(value),
	)).Run()
}

// promptInputValidated is promptInput with a validator (e.g. a date or number).
func promptInputValidated(title string, value *string, validate func(string) error) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Title(title).Value(value).Validate(validate),
	)).Run()
}

// promptSelect asks the user to pick one of the given string options.
func promptSelect(title string, value *string, options ...huh.Option[string]) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title(title).Options(options...).Value(value),
	)).Run()
}
