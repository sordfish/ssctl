package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"ssctl/pkg/kube"
	"ssctl/pkg/sunsynk"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {

		debugFlagValue, _ := cmd.Parent().PersistentFlags().GetBool("debug")

		if debugFlagValue {
			os.Setenv("SS_DEBUG", "TRUE")
		}

		k8sFlagValue, _ := cmd.Parent().PersistentFlags().GetBool("k8s")
		Auth(k8sFlagValue)
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// authCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// authCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

func Auth(k8s bool) {

	var GetNewAuthTokenResponse sunsynk.SSApiNewTokenResponse

	namespace := os.Getenv("SS_NAMESPACE")
	if namespace == "" {
		namespace = "sunsynk"
	}

	if !k8s {

		SunsynkUser := os.Getenv("SS_USER")
		SunsynkPass := os.Getenv("SS_PASS")

		if SunsynkUser == "" || SunsynkPass == "" {
			log.Fatal("No credentials found in env")
		}

		GetNewAuthTokenResponse, err := sunsynk.GetNewAuthToken(string(SunsynkUser), string(SunsynkPass))
		if err != nil {
			log.Fatal(err)
		}

		TokenJson, err := json.Marshal(GetNewAuthTokenResponse)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(TokenJson))

	} else {

		clientset, err := kube.Login()
		if err != nil {
			log.Fatal(err)
		}

		// Get the "default" namespace

		// Get the credential secret
		result, err := kube.GetK8sSecret(clientset, "sunsynk-credentials", namespace)
		if err != nil {
			log.Fatal(err)
		}

		username, ok := result.Data["username"]
		if !ok {
			log.Fatal("username not found in secret data")
		}

		password, ok := result.Data["password"]
		if !ok {
			log.Fatal("password not found in secret data")
		}

		GetNewAuthTokenResponse, err = sunsynk.GetNewAuthToken(string(username), string(password))
		if err != nil {
			log.Fatal(err)
		}

		// Create a new secret
		SunsynkTokenSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sunsynk-token",
				Namespace: namespace,
			},
			Type: "Opaque",
			Data: map[string][]byte{
				"token":     []byte(GetNewAuthTokenResponse.Data.AccessToken),
				"type":      []byte(GetNewAuthTokenResponse.Data.TokenType),
				"refresh":   []byte(GetNewAuthTokenResponse.Data.RefreshToken),
				"expiry":    []byte(fmt.Sprint(GetNewAuthTokenResponse.Data.TokenExpiry)),
				"scope":     []byte(GetNewAuthTokenResponse.Data.Scope),
				"timestamp": []byte(fmt.Sprint(time.Now().Unix())),
			},
		}

		// Get the secret
		result, err = kube.GetK8sSecret(clientset, "sunsynk-token", "sunsynk")
		if err != nil {
			//Create the secret
			result, err = clientset.CoreV1().Secrets(namespace).Create(context.Background(), SunsynkTokenSecret, metav1.CreateOptions{})
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Created secret %q.\n", result.GetObjectMeta().GetName())
		} else {
			// Update the secret
			result.Data["token"] = []byte(GetNewAuthTokenResponse.Data.AccessToken)
			result.Data["type"] = []byte(GetNewAuthTokenResponse.Data.TokenType)
			result.Data["refresh"] = []byte(GetNewAuthTokenResponse.Data.RefreshToken)
			result.Data["expiry"] = []byte(fmt.Sprint(GetNewAuthTokenResponse.Data.TokenExpiry))
			result.Data["scope"] = []byte(GetNewAuthTokenResponse.Data.Scope)
			result.Data["timestamp"] = []byte(fmt.Sprint(time.Now().Unix()))

			_, err = clientset.CoreV1().Secrets(namespace).Update(context.Background(), result, metav1.UpdateOptions{})
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Updated secret %q.\n", result.GetObjectMeta().GetName())
		}
	}
}
