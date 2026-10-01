package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"xingdu.app/xingdu/internal/billing"
	"xingdu.app/xingdu/internal/bootstrap"
	"xingdu.app/xingdu/internal/certificates"
	"xingdu.app/xingdu/internal/config"
	"xingdu.app/xingdu/internal/emailverification"
	"xingdu.app/xingdu/internal/httpapi"
	"xingdu.app/xingdu/internal/socialauth"
	"xingdu.app/xingdu/internal/storage"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	store, err := storage.OpenRuntime(startup, cfg.DatabaseURL, cfg.Mode)
	cancel()
	if err != nil {
		return err
	}
	defer store.Close()
	readiness, readyCancel := context.WithTimeout(ctx, 10*time.Second)
	readyErr := store.Ready(readiness)
	readyCancel()
	if readyErr != nil {
		return errors.New("database migrations incomplete or database unavailable; apply migrations before starting API")
	}
	connector, credentialVault, agentOrigin, err := bootstrap.Configuration(cfg.PublicOrigin)
	if err != nil {
		return err
	}
	billingConfig := billing.Config{Mode: cfg.Mode, SecretKey: os.Getenv("STRIPE_SECRET_KEY"), WebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"), MonthlyPrice: os.Getenv("STRIPE_PRICE_MONTHLY"), YearlyPrice: os.Getenv("STRIPE_PRICE_YEARLY"), PremiumMonthlyPrice: os.Getenv("STRIPE_PRICE_PREMIUM_MONTHLY"), PremiumYearlyPrice: os.Getenv("STRIPE_PRICE_PREMIUM_YEARLY"), PortalConfiguration: os.Getenv("STRIPE_PORTAL_CONFIGURATION")}
	if err := billingConfig.Validate(); err != nil {
		return err
	}
	if billingConfig.Cloud() && billingConfig.Live() && !cfg.SecureCookies {
		return errors.New("live Stripe billing requires an HTTPS public origin")
	}
	store.CloudBilling = billingConfig.Cloud()
	var gateway billing.Gateway
	if billingConfig.Cloud() {
		gateway = billing.New(billingConfig)
	}
	mailer := emailverification.NewResend(os.Getenv("RESEND_API_KEY"), os.Getenv("XINGDU_EMAIL_FROM"))
	go store.RunEmailDelivery(ctx, mailer, cfg.PublicOrigin)
	certificateConfig := certificates.ManagedEnvironment()
	certificateConfig.LoadAccountKey = func(accountCtx context.Context) (*ecdsa.PrivateKey, error) {
		return store.ACMEAccountKey(accountCtx, credentialVault, certificateConfig.Directory, certificateConfig.StateDir)
	}
	if certificateConfig.Configured() {
		go func() {
			initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			_, err := certificateConfig.LoadAccountKey(initCtx)
			cancel()
			if err != nil {
				slog.Warn("ACME account initialization unavailable")
			}
			store.RunCertificateDelivery(ctx, certificateConfig, credentialVault)
		}()
	}
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.New(store, httpapi.Options{Certificates: certificateConfig, Mode: cfg.Mode, Billing: gateway, BillingCloud: billingConfig.Cloud(), BillingPremium: billingConfig.PremiumMonthlyPrice != "" && billingConfig.PremiumYearlyPrice != "", BillingTest: billingConfig.Cloud() && !billingConfig.Live(), OAuthProviders: socialauth.New(socialauth.Config{Origin: cfg.PublicOrigin, GoogleClientID: os.Getenv("XINGDU_GOOGLE_CLIENT_ID"), GoogleClientSecret: os.Getenv("XINGDU_GOOGLE_CLIENT_SECRET"), GitHubClientID: os.Getenv("XINGDU_GITHUB_CLIENT_ID"), GitHubClientSecret: os.Getenv("XINGDU_GITHUB_CLIENT_SECRET")}), EmailSender: mailer, PublicOrigin: cfg.PublicOrigin, SecureCookies: cfg.SecureCookies, RegistrationEnabled: os.Getenv("XINGDU_REGISTRATION_ENABLED") == "true", AgentOrigin: agentOrigin, ArtifactDir: connector.ArtifactDir, CredentialVault: credentialVault, SSHConnector: connector}), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	slog.Info("xingdu API starting", "address", cfg.HTTPAddr, "stage", "multi-tenant")
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}
