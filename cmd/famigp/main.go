// Comando famigp arranca la aplicación web que convierte partituras de
// Guitar Pro en música chiptune.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Marcusdeks/FamiGp/internal/server"
	"github.com/Marcusdeks/FamiGp/web"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "dirección y puerto de escucha")
	flag.Parse()

	// Sin la interfaz compilada se sirve solo la API (modo desarrollo).
	static, err := web.Files()
	if err != nil {
		log.Printf("aviso: %v", err)
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           server.New(static),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Al recibir Ctrl+C o SIGTERM (p. ej. al parar el contenedor) se deja
	// terminar a las peticiones en curso antes de salir.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("cierre forzado: %v", err)
		}
	}()

	log.Printf("FamiGP disponible en http://%s", *addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
