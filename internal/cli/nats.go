package cli

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newNATSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nats",
		Short: "Publish and subscribe to NATS subjects",
	}

	cmd.AddCommand(newNATSPublishCmd(), newNATSSubscribeCmd())
	return cmd
}

func newNATSPublishCmd() *cobra.Command {
	var subject string

	cmd := &cobra.Command{
		Use:   "publish --subject <subject> <message>",
		Short: "Publish a message to NATS",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newNATSClient(viper.GetString("nats-url"))
			if err != nil {
				return err
			}
			defer client.Close()

			if err := client.Publish(cmd.Context(), subject, []byte(args[0])); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Published message to %s\n", subject)
			return err
		},
	}

	cmd.Flags().StringVar(&subject, "subject", "", "Subject to publish to")
	_ = cmd.MarkFlagRequired("subject")
	return cmd
}

func newNATSSubscribeCmd() *cobra.Command {
	var subject string
	var count int

	cmd := &cobra.Command{
		Use:   "subscribe --subject <subject>",
		Short: "Subscribe to a NATS subject",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newNATSClient(viper.GetString("nats-url"))
			if err != nil {
				return err
			}
			defer client.Close()

			return client.Subscribe(cmd.Context(), subject, count, func(msg *nats.Msg) error {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", msg.Subject, string(msg.Data))
				return err
			})
		},
	}

	cmd.Flags().StringVar(&subject, "subject", "", "Subject to subscribe to")
	cmd.Flags().IntVar(&count, "count", 0, "Stop after receiving this many messages (0 means keep listening)")
	_ = cmd.MarkFlagRequired("subject")
	return cmd
}
