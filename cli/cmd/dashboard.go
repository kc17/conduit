package cmd

import (
	"fmt"
	"sync"

	"github.com/pkg/browser"
	"github.com/runconduit/conduit/pkg/k8s"
	"github.com/runconduit/conduit/pkg/shell"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/proxy"
	// required to authenticate against GKE clusters
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	proxyPort = -1
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard [flags]",
	Short: "Open the Conduit dashboard in a web browser",
	Long:  "Open the Conduit dashboard in a web browser.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if proxyPort <= 0 {
			log.Fatalf("port must be positive, was %d", proxyPort)
		}

		shell := shell.NewUnixShell()
		kubectl, err := k8s.NewKubectl(shell)
		if err != nil {
			log.Fatalf("Failed to start kubectl: %v", err)
		}

		clientConfig, err := k8s.NewK8sRestConfig(kubeconfigPath, shell.HomeDir())
		if err != nil {
			log.Fatalf("NewK8sRestConfig() failed with: %+v", err)
		}

		filter := &proxy.FilterServer{
			AcceptPaths:   proxy.MakeRegexpArrayOrDie(proxy.DefaultPathAcceptRE),
			RejectPaths:   proxy.MakeRegexpArrayOrDie(proxy.DefaultPathRejectRE),
			AcceptHosts:   proxy.MakeRegexpArrayOrDie(proxy.DefaultHostAcceptRE),
			RejectMethods: proxy.MakeRegexpArrayOrDie(proxy.DefaultMethodRejectRE),
		}
		server, err := proxy.NewServer("", "/", "/static/", filter, clientConfig)
		if err != nil {
			log.Fatalf("proxy.NewServer() failed with: %+v", err)
		}
		l, err := server.Listen("127.0.0.1", proxyPort)
		if err != nil {
			log.Fatalf("server.Listen() failed with: %+v", err)
		}

		// TODO: log without formatting for information stuff
		log.Infof("Starting to serve on %s\n", l.Addr().String())

		// go func(s *proxy.Server) { log.Fatal(s.ServeOnListener(l)) }(server)
		go server.ServeOnListener(l)

		log.Infof("FOO %s\n", l.Addr().String())

		url, err := kubectl.UrlFor(controlPlaneNamespace, "/services/web:http/proxy/")
		if err != nil {
			log.Fatalf("Failed to generate URL for dashboard: %v", err)
		}

		fmt.Printf("Opening [%s] in the default browser\n", url)
		err = browser.OpenURL(url.String())
		if err != nil {
			// log.Fatalf("failed to open URL %s in the default browser: %v", url, err)
		}

		wg := sync.WaitGroup{}
		wg.Add(1)
		wg.Wait()

		// blocks
		// log.Fatal(server.ServeOnListener(l))

		// select {
		// case err = <-asyncProcessErr:
		// 	if err != nil {
		// 		log.Fatalf("Error starting proxy via kubectl: %v", err)
		// 	}
		// }
		// close(asyncProcessErr)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(dashboardCmd)
	addControlPlaneNetworkingArgs(dashboardCmd)
	dashboardCmd.Args = cobra.NoArgs

	// This is identical to what `kubectl proxy --help` reports, except
	// `kubectl proxy` allows `--port=0` to indicate a random port; That's
	// inconvenient to support so it isn't supported.
	dashboardCmd.PersistentFlags().IntVarP(&proxyPort, "port", "p", 8001, "The port on which to run the proxy, which must not be 0.")
}
