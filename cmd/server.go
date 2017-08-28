package cmd

import (
	"context"

	"github.com/nicwest/moist/server"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "runs a moist mail server",
	Long: `Runs a moist SMTP mail server.
	
	Accepts incoming mail from external sources and stores them in the local 
	store. 

	Accepts incoming mail from a known source and forwards it on to the intended
	recipient
	`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		cctx, cancel := context.WithCancel(ctx)
		defer cancel()

		addr := viper.GetString("server_addr")
		domain := viper.GetString("server_domain")

		if domain == "" {
			log.Fatal("no server domain specified")
		}

		fields := log.Fields{
			"domain": domain,
			"addr":   addr,
		}

		log.WithFields(fields).Info("starting server")
		s := server.New(domain)

		go func() {
			err := s.Listen(cctx, addr)
			if err != nil {
				log.Fatal(err)
			}
		}()

		<-s.Ready
		log.WithFields(fields).Info("server started!")

		for {
			select {
			case <-cctx.Done():
				log.Info("bye bye!")
				return

			case msg := <-s.Inbox:
				log.Infof("%+v", msg)
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(serverCmd)
	serverCmd.PersistentFlags().String("db", "moist.db", "the location of the moist database")
	serverCmd.PersistentFlags().String("domain", "", "the domain of the server")
	serverCmd.PersistentFlags().String("addr", ":1025", "the address to bind")
	viper.BindPFlag("server_db", serverCmd.Flags().Lookup("db"))
	viper.BindPFlag("server_domain", serverCmd.Flags().Lookup("domain"))
	viper.BindPFlag("server_addr", serverCmd.Flags().Lookup("addr"))
}
