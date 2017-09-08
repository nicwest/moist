package cmd

import (
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

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

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			sig := <-sigs
			log.WithFields(
				log.Fields{
					"signal": sig,
				}).Info("received termination signal")
			cancel()
		}()

		addr := viper.GetString("addr")
		domain := viper.GetString("domain")

		if domain == "" {
			log.Fatal("no server domain specified")
		}

		fields := log.Fields{
			"domain": domain,
			"addr":   addr,
		}

		log.WithFields(fields).Info("starting server")
		s := server.New(domain)
		s.ToWhiteList = []string{"foo@bar.com"}

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
				log.Infof("%+v", msg.Message)
				if msg.Message != nil {
					body, err := ioutil.ReadAll(msg.Message.Body)
					if err != nil {
						log.Error(err)
					}
					log.Infof("%+v", string(body))
				}
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(serverCmd)
	serverCmd.Flags().String("db", "moist.db", "the location of the moist database")
	serverCmd.Flags().String("domain", "", "the domain of the server")
	serverCmd.Flags().String("addr", ":1025", "the address to bind")
	viper.BindPFlag("db", serverCmd.Flags().Lookup("db"))
	viper.BindPFlag("domain", serverCmd.Flags().Lookup("domain"))
	viper.BindPFlag("addr", serverCmd.Flags().Lookup("addr"))
}
