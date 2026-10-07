package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
)

func main() {
	output := flag.String("output", "", "Output PDF path; defaults to output/pdf/<scope>-webhook-api-manual.pdf")
	scopeValue := flag.String("scope", "", "Required manual scope: asset_library or media_tasks")
	flag.StringVar(&common.SystemName, "platform-name", "星枢 MaaS 平台", "Platform name")
	flag.StringVar(&common.Logo, "platform-logo", "", "HTTP(S) platform logo URL; empty uses the bundled mark")
	flag.StringVar(&common.OperatingEntityName, "operating-name", "", "Operating entity name")
	flag.StringVar(&common.OperatingEntityLogo, "operating-logo", "", "HTTP(S) operating entity logo URL")
	flag.StringVar(&common.Footer, "footer", "NovaMaaS | 基于 QuantumNous/new-api | Webhook 接口手册", "Visible document footer")
	flag.Parse()
	scope := service.WebhookManualScope(*scopeValue)
	if !scope.Valid() {
		fmt.Fprintln(os.Stderr, "scope must be asset_library or media_tasks")
		os.Exit(2)
	}
	if *output == "" {
		*output = filepath.Join("output", "pdf", string(scope)+"-webhook-api-manual.pdf")
	}
	service.InitHttpClient()
	pdf, err := service.RenderWebhookManualPDF(context.Background(), scope)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(*output), 0755)
	}
	if err == nil {
		err = os.WriteFile(*output, pdf, 0644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(*output)
}
