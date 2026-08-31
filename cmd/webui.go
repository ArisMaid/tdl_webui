package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/iyear/tdl/app/webui"
	"github.com/iyear/tdl/core/logctx"
)

func NewWebui() *cobra.Command {
	var (
		host  string
		port  int
		token string
	)

	cmd := &cobra.Command{
		Use:     "webui",
		Short:   "Start the WebUI server to manage tdl from your browser",
		GroupID: groupTools.ID,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := webui.New(webui.Options{
				Host:  host,
				Port:  port,
				Token: token,
			})
			if err != nil {
				return err
			}
			return srv.Run(logctx.Named(cmd.Context(), "webui"))
		},
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "listen address of the WebUI server")
	cmd.Flags().IntVar(&port, "port", 8080, "listen port of the WebUI server")
	cmd.Flags().StringVar(&token, "token", "",
		"bearer token to protect the WebUI, empty means no authentication (env: TDL_WEBUI_TOKEN)")

	// fallback to environment variable
	if token == "" {
		token = os.Getenv("TDL_WEBUI_TOKEN")
	}

	return cmd
}
