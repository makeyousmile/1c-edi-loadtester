package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"loadtester/internal/mock1c"
	"loadtester/internal/tester"
	"loadtester/internal/web"
)

func main() {
	port := flag.Int("port", 8080, "Порт для веб-интерфейса сервиса нагрузочного тестирования")
	templatePath := flag.String("template", "blrwbl 517_001-4810413900004-260616000002408.xml", "Путь к эталонному XML-шаблону BLRWBL")
	flag.Parse()

	// Read XML template
	templateBytes, err := os.ReadFile(*templatePath)
	if err != nil {
		log.Fatalf("Ошибка чтения XML шаблона %s: %v", *templatePath, err)
	}
	templateXML := string(templateBytes)

	// Initialize components
	mockServer := mock1c.NewMockServer()
	runner := tester.NewRunner(templateXML)
	server := web.NewServer(runner, mockServer, templateXML)

	addr := fmt.Sprintf(":%d", *port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	// Graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Println("===================================================================")
		fmt.Println("  🚀 Сервис нагрузочного тестирования 1С BLRWBL запущен")
		fmt.Printf("  🌐 Веб-интерфейс:     http://localhost:%d/\n", *port)
		fmt.Printf("  📦 Шаблон XML:        %s\n", *templatePath)
		fmt.Printf("  🔌 Встроенный Mock 1C: http://localhost:%d/mock/1c/api/v1/edocuments\n", *port)
		fmt.Println("===================================================================")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка HTTP сервера: %v", err)
		}
	}()

	<-stopChan
	fmt.Println("\nЗавершение работы сервиса...")

	runner.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Ошибка при остановке сервера: %v", err)
	}
	fmt.Println("Сервис успешно остановлен.")
}
