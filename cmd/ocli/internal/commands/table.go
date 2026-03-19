package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	runtimepkg "github.com/StevenBuglione/open-cli/cmd/ocli/internal/runtime"
	"github.com/StevenBuglione/open-cli/pkg/catalog"
)

// IsTerminal reports whether w is connected to a terminal device.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// WriteTable renders value as a human-readable table.
func WriteTable(out io.Writer, value any) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	switch v := value.(type) {
	case runtimepkg.CatalogResponse:
		return writeCatalogTable(w, v)
	case *catalog.Tool:
		return writeToolTable(w, v)
	case catalog.Tool:
		return writeToolTable(w, &v)
	case map[string]any:
		return writeMapTable(w, v)
	default:
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		_, err = out.Write(append(data, '\n'))
		return err
	}
}

func writeCatalogTable(w *tabwriter.Writer, resp runtimepkg.CatalogResponse) error {
	serviceAliases := map[string]string{}
	for _, svc := range resp.Catalog.Services {
		serviceAliases[svc.ID] = svc.Alias
	}
	fmt.Fprintf(w, "SERVICE\tGROUP\tCOMMAND\tMETHOD\tSUMMARY\n")
	for _, tool := range resp.View.Tools {
		alias := serviceAliases[tool.ServiceID]
		if alias == "" {
			alias = tool.ServiceID
		}
		summary := tool.Summary
		if summary == "" {
			summary = tool.Description
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", alias, tool.Group, tool.Command, tool.Method, summary)
	}
	return w.Flush()
}

func writeToolTable(w *tabwriter.Writer, tool *catalog.Tool) error {
	fmt.Fprintf(w, "FIELD\tVALUE\n")
	fmt.Fprintf(w, "ID\t%s\n", tool.ID)
	fmt.Fprintf(w, "Service\t%s\n", tool.ServiceID)
	fmt.Fprintf(w, "Method\t%s\n", tool.Method)
	fmt.Fprintf(w, "Path\t%s\n", tool.Path)
	fmt.Fprintf(w, "Group\t%s\n", tool.Group)
	fmt.Fprintf(w, "Command\t%s\n", tool.Command)
	if tool.Summary != "" {
		fmt.Fprintf(w, "Summary\t%s\n", tool.Summary)
	}
	if tool.Description != "" {
		fmt.Fprintf(w, "Description\t%s\n", tool.Description)
	}
	return w.Flush()
}

func writeMapTable(w *tabwriter.Writer, m map[string]any) error {
	fmt.Fprintf(w, "KEY\tVALUE\n")
	for k, v := range m {
		fmt.Fprintf(w, "%s\t%v\n", k, v)
	}
	return w.Flush()
}
