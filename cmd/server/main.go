package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/faeln1/go-whatsapp-api/internal/app/controllers"
	"github.com/faeln1/go-whatsapp-api/internal/app/repositories"
	"github.com/faeln1/go-whatsapp-api/internal/app/services"
	"github.com/faeln1/go-whatsapp-api/internal/config"
	"github.com/faeln1/go-whatsapp-api/internal/domain/instance"
	"github.com/faeln1/go-whatsapp-api/internal/platform/database"
	httpPlatform "github.com/faeln1/go-whatsapp-api/internal/platform/http"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"github.com/faeln1/go-whatsapp-api/pkg/eventlog"
	"github.com/faeln1/go-whatsapp-api/pkg/logger"
	storagepkg "github.com/faeln1/go-whatsapp-api/pkg/storage"
	minioStorage "github.com/faeln1/go-whatsapp-api/pkg/storage/minio"
	"github.com/joho/godotenv"
	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"
	"gorm.io/gorm"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("warning: could not load .env: %v", err)
	}

	cfg := config.MustLoad()
	loggers := logger.New("DEBUG")

	log.Printf("configuration: driver=%s dsn=%s", cfg.DBDriver, cfg.DatabaseDSN)

	var objectStorage storagepkg.Service
	if cfg.Storage.Enabled() {
		store, err := minioStorage.New(context.Background(), minioStorage.Config{
			Endpoint:  cfg.Storage.Endpoint,
			AccessKey: cfg.Storage.AccessKey,
			SecretKey: cfg.Storage.SecretKey,
			Bucket:    cfg.Storage.Bucket,
			Region:    cfg.Storage.Region,
			UseSSL:    cfg.Storage.UseSSL,
			PublicURL: cfg.Storage.PublicURL,
		})
		if err != nil {
			log.Fatalf("storage initialization error: %v", err)
		}
		objectStorage = store
		log.Printf("object storage enabled bucket=%s endpoint=%s", cfg.Storage.Bucket, cfg.Storage.Endpoint)
	}

	var (
		repo           repositories.InstanceRepository
		membershipRepo repositories.CommunityMembershipRepository
		analyticsRepo  repositories.AnalyticsRepository
		db             *gorm.DB
		dbClose        func() error
	)

	switch cfg.DBDriver {
	case "postgres":
		log.Printf("initializing postgres repository with GORM")
		var err error
		db, err = database.Open(cfg.DatabaseDSN)
		if err != nil {
			log.Fatalf("database connection error: %v", err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			log.Fatalf("database handle retrieval error: %v", err)
		}
		dbClose = sqlDB.Close

		gormRepo, err := repositories.NewGormInstanceRepo(db)
		if err != nil {
			log.Fatalf("repository initialization error: %v", err)
		}
		repo = gormRepo

		// TODO: Refatorar os outros repositórios para GORM também
		membershipRepo, err = repositories.NewPostgresCommunityMembershipRepo(sqlDB)
		if err != nil {
			log.Fatalf("membership repository initialization error: %v", err)
		}
		analyticsRepo = repositories.NewAnalyticsRepository(sqlDB)
	default:
		log.Printf("initializing in-memory repository")
		repo = repositories.NewInMemoryInstanceRepo()
		membershipRepo = repositories.NewInMemoryCommunityMembershipRepo()
	}
	if membershipRepo == nil {
		membershipRepo = repositories.NewInMemoryCommunityMembershipRepo()
	}

	if dbClose != nil {
		defer func() {
			if err := dbClose(); err != nil {
				log.Printf("error closing database: %v", err)
			}
		}()
	}

	waMgr := whatsapp.NewManager(loggers.App.Sub("WA"))
	storeFactory := whatsapp.NewStoreFactory(cfg.DataDir, loggers.App.Sub("Store"))
	webhookDispatcher := services.NewWebhookDispatcher(nil, loggers.App.Sub("Webhook"))
	communityEventsDispatcher := services.NewCommunityEventsDispatcher(cfg.CommunityEventsWebhookURL, cfg.CommunityEventsToken, nil, loggers.App.Sub("CommunityWebhook"))

	var analyticsSvc services.AnalyticsService
	if analyticsRepo != nil {
		analyticsSvc = services.NewAnalyticsService(analyticsRepo)
	}

	messageEvents := services.NewMessageEventHandler(repo, waMgr, objectStorage, webhookDispatcher, analyticsSvc, loggers.App.Sub("Events"))
	communityEvents := services.NewCommunityEventService(waMgr, membershipRepo, communityEventsDispatcher, loggers.App.Sub("CommunityEvents"))
	eventLogger := eventlog.NewWriter(cfg.EventLogDir, loggers.App.Sub("EventLog"))
	bootstrap := services.NewSessionBootstrap(storeFactory, waMgr, loggers.App.Sub("Bootstrap"), messageEvents, eventLogger)
	bootstrap.ReceiptEvents = messageEvents
	bootstrap.GroupEvents = communityEvents

	instanceSvc := services.NewInstanceService(repo, waMgr, objectStorage)
	messageSvc := services.NewMessageService(waMgr, objectStorage)
	communitySvc := services.NewCommunityService(waMgr, messageSvc, analyticsSvc, membershipRepo)
	groupSvc := services.NewGroupService(waMgr)
	profileSvc := services.NewProfileService(waMgr)

	if cfg.DBDriver == "postgres" {
		restoreInstances(context.Background(), repo, instanceSvc, bootstrap, waMgr, loggers.App.Sub("Restore"))
	}

	instanceCtrl := controllers.NewInstanceController(instanceSvc, bootstrap, webhookDispatcher)
	messageCtrl := controllers.NewMessageController(messageSvc)
	communityCtrl := controllers.NewCommunityController(communitySvc)
	groupCtrl := controllers.NewGroupController(groupSvc)
	webhookCtrl := controllers.NewWebhookController(instanceSvc)
	settingsCtrl := controllers.NewSettingsController(instanceSvc)
	profileCtrl := controllers.NewProfileController(profileSvc)

	var analyticsCtrl *controllers.AnalyticsController
	if analyticsSvc != nil {
		analyticsCtrl = controllers.NewAnalyticsController(analyticsSvc)
	}

	router := httpPlatform.NewRouter(httpPlatform.RouterConfig{
		InstanceCtrl:  instanceCtrl,
		MessageCtrl:   messageCtrl,
		CommunityCtrl: communityCtrl,
		GroupCtrl:     groupCtrl,
		WebhookCtrl:   webhookCtrl,
		SettingsCtrl:  settingsCtrl,
		ProfileCtrl:   profileCtrl,
		AnalyticsCtrl: analyticsCtrl,
		Logger:        loggers.HTTP,
		WAManager:     waMgr,
		SwaggerEnable: cfg.SwaggerEnable,
		MasterToken:   cfg.MasterToken,
	})

	srv := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: router}
	go func() {
		log.Printf("HTTP server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")
	_ = srv.Shutdown(context.Background())
}

func restoreInstances(ctx context.Context, repo repositories.InstanceRepository, instanceSvc services.InstanceService, bootstrap *services.SessionBootstrap, waMgr *whatsapp.Manager, log waLog.Logger) {
	instances, err := repo.List(ctx)
	if err != nil {
		log.Errorf("failed to restore instances: %v", err)
		return
	}
	if len(instances) == 0 {
		log.Infof("no instances to restore")
		return
	}
	log.Infof("restoring %d instance(s)", len(instances))
	for _, inst := range instances {
		inst := inst
		go restoreInstance(ctx, inst, instanceSvc, bootstrap, waMgr, log.Sub(inst.Name))
	}
}

func restoreInstance(ctx context.Context, inst *instance.Instance, instanceSvc services.InstanceService, bootstrap *services.SessionBootstrap, waMgr *whatsapp.Manager, log waLog.Logger) {
	sess, err := waMgr.Create(ctx, inst.Name, inst.Token)
	if err != nil {
		if errors.Is(err, whatsapp.ErrAlreadyExists) {
			log.Warnf("session already registered in manager")
		} else {
			log.Errorf("failed to register session in manager: %v", err)
			return
		}
	} else {
		sess.ID = string(inst.ID)
		sess.CreatedAt = inst.CreatedAt
		sess.Token = inst.Token
	}

	qrChan, alreadyLogged, err := bootstrap.InitNewSession(context.Background(), inst.Name)
	if err != nil {
		log.Errorf("failed to initialize whatsapp session: %v", err)
		return
	}
	if alreadyLogged {
		log.Infof("session restored (already logged in)")
		return
	}
	if qrChan == nil {
		log.Infof("session requires login but QR channel not available")
		return
	}

	log.Infof("session requires QR scan; waiting for events")
	watchQRChannel(ctx, inst.Name, qrChan, instanceSvc, log)
}

func watchQRChannel(ctx context.Context, instanceName string, qrChan <-chan whatsmeow.QRChannelItem, instanceSvc services.InstanceService, log waLog.Logger) {
	for {
		select {
		case item, ok := <-qrChan:
			if !ok {
				log.Infof("QR channel closed")
				return
			}
			switch item.Event {
			case "code":
				if item.Code == "" {
					continue
				}
				if link, err := instanceSvc.CacheQRCode(context.Background(), instanceName, item.Code); err != nil {
					log.Errorf("failed to cache QR code: %v", err)
				} else {
					if link != "" {
						log.Infof("QR code refreshed (timeout %s) link=%s", item.Timeout, link)
					} else {
						log.Infof("QR code refreshed (timeout %s)", item.Timeout)
					}
					whatsapp.PrintQRASCII(item.Code)
				}
			case "success":
				log.Infof("device paired successfully")
				return
			case "timeout":
				log.Warnf("QR code timeout received")
				return
			case "error":
				log.Errorf("error event from QR channel")
			default:
				log.Infof("received QR event: %s", item.Event)
			}
		case <-ctx.Done():
			log.Warnf("context cancelled while waiting for QR events")
			return
		}
	}
}
