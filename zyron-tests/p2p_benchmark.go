package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"
)

// Estrutura do pacote de teste UDP (simula pacote RTP de áudio/vídeo WebRTC)
// [8 bytes: SeqNum] [8 bytes: TimestampNs (Envio)] [8 bytes: TimestampNs (Echo/Server)] [Payload restante preenchido]
const (
	headerSize = 24
)

type BenchmarkConfig struct {
	Mode       string
	Port       int
	TargetAddr string
	PacketSize int
	PacketRate int
	Duration   time.Duration
}

type PacketStats struct {
	SeqNum    uint64
	RTT       time.Duration
	ServerRec time.Duration
}

func main() {
	cfg := BenchmarkConfig{}

	flag.StringVar(&cfg.Mode, "mode", "client", "Modo de execução: 'server' (no operador) ou 'client' (no cliente)")
	flag.IntVar(&cfg.Port, "port", 19800, "Porta UDP para escuta ou envio")
	flag.StringVar(&cfg.TargetAddr, "target", "127.0.0.1", "IP de destino da outra ponta (usado no modo client)")
	flag.IntVar(&cfg.PacketSize, "size", 1200, "Tamanho do pacote UDP em bytes (padrão RTP WebRTC: 1200)")
	flag.IntVar(&cfg.PacketRate, "rate", 60, "Taxa de pacotes por segundo (ex: 30 = ~300kbps, 60 = ~600kbps, 120 = ~1.2mbps)")
	flag.DurationVar(&cfg.Duration, "duration", 30*time.Second, "Duração do teste no modo client")
	flag.Parse()

	if cfg.PacketSize < headerSize {
		cfg.PacketSize = headerSize
	}

	fmt.Println("================================================================================")
	fmt.Println("             ZYRON P2P - TESTE DE QUALIDADE DE REDE UDP (WEBRTC)               ")
	fmt.Println("================================================================================")

	if cfg.Mode == "server" {
		runUDPServer(cfg)
	} else if cfg.Mode == "client" {
		runUDPClient(cfg)
	} else {
		fmt.Printf("Modo inválido: %s. Utilize -mode server ou -mode client\n", cfg.Mode)
		os.Exit(1)
	}
}

// runUDPServer escuta na porta e reflete os pacotes de volta imediatamente com timestamp de retorno
func runUDPServer(cfg BenchmarkConfig) {
	listenAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	conn, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
		fmt.Printf("Erro ao abrir porta UDP %s: %v\n", listenAddr, err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf(" [SERVIDOR P2P ATIVO] Aguardando conexões e pacotes na porta UDP %d...\n", cfg.Port)
	fmt.Println(" Pressione Ctrl+C para encerrar.")
	fmt.Println("================================================================================")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nEncerrando servidor UDP...")
		conn.Close()
		os.Exit(0)
	}()

	buffer := make([]byte, 65535)
	var totalPacketsEchoed uint64
	startTime := time.Now()

	for {
		n, clientAddr, err := conn.ReadFrom(buffer)
		if err != nil {
			return
		}

		if n >= headerSize {
			nowNs := uint64(time.Now().UnixNano())
			binary.BigEndian.PutUint64(buffer[16:24], nowNs) // Timestamp do servidor
			_, _ = conn.WriteTo(buffer[:n], clientAddr)
			totalPacketsEchoed++

			if totalPacketsEchoed%500 == 0 {
				elapsed := time.Since(startTime).Round(time.Second)
				fmt.Printf("[%s] Pacotes refletidos (Echo): %d | Último cliente: %s\n", elapsed, totalPacketsEchoed, clientAddr.String())
			}
		}
	}
}

// runUDPClient envia pacotes UDP para o servidor e calcula RTT, Jitter (RFC 3550) e Perda
func runUDPClient(cfg BenchmarkConfig) {
	target := fmt.Sprintf("%s:%d", cfg.TargetAddr, cfg.Port)
	raddr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		fmt.Printf("Erro ao resolver endereço de destino %s: %v\n", target, err)
		os.Exit(1)
	}

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		fmt.Printf("Erro ao conectar UDP com %s: %v\n", target, err)
		os.Exit(1)
	}
	defer conn.Close()

	bandwidthKbps := (float64(cfg.PacketSize*8) * float64(cfg.PacketRate)) / 1000.0

	fmt.Printf(" Destino P2P (Operador) : %s\n", target)
	fmt.Printf(" Tamanho do Pacote      : %d bytes (RTP payload típico)\n", cfg.PacketSize)
	fmt.Printf(" Taxa de Envio          : %d pacotes/s (~%.1f Kbps de largura de banda)\n", cfg.PacketRate, bandwidthKbps)
	fmt.Printf(" Duração do Teste       : %v\n", cfg.Duration)
	fmt.Println("================================================================================")

	packetInterval := time.Second / time.Duration(cfg.PacketRate)
	sendPayload := make([]byte, cfg.PacketSize)

	var (
		mu           sync.Mutex
		packetsSent  uint64
		packetsRecv  uint64
		samples      []PacketStats
		prevTransit  float64
		jitterRFC3550 float64 // Jitter acumulado RFC 3550 (ms)
		outOfOrder   uint64
		lastSeqRecv  uint64
	)

	stopChan := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Rotina de Recepção dos Echos
	go func() {
		recvBuf := make([]byte, 65535)
		for {
			select {
			case <-stopChan:
				return
			default:
				_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				n, err := conn.Read(recvBuf)
				if err != nil {
					continue
				}
				if n >= headerSize {
					recvTime := time.Now()
					seqNum := binary.BigEndian.Uint64(recvBuf[0:8])
					sendTimeNs := int64(binary.BigEndian.Uint64(recvBuf[8:16]))
					sendTime := time.Unix(0, sendTimeNs)
					rtt := recvTime.Sub(sendTime)

					mu.Lock()
					packetsRecv++
					samples = append(samples, PacketStats{
						SeqNum: seqNum,
						RTT:    rtt,
					})

					if seqNum < lastSeqRecv {
						outOfOrder++
					} else {
						lastSeqRecv = seqNum
					}

					// Cálculo do Jitter conforme RFC 3550 (WebRTC / VoIP standard):
					// D(i, j) = (R_j - S_j) - (R_i - S_i)
					// J_i = J_{i-1} + (|D(i, j)| - J_{i-1}) / 16
					transitTime := float64(rtt.Microseconds()) / 1000.0 // ms
					if prevTransit > 0 {
						d := math.Abs(transitTime - prevTransit)
						jitterRFC3550 += (d - jitterRFC3550) / 16.0
					}
					prevTransit = transitTime

					mu.Unlock()
				}
			}
		}
	}()

	fmt.Println("[*] Enviando fluxo de pacotes UDP...")
	startTime := time.Now()
	ticker := time.NewTicker(packetInterval)
	defer ticker.Stop()

	testTimer := time.NewTimer(cfg.Duration)
	defer testTimer.Stop()

	var seq uint64

loop:
	for {
		select {
		case <-sigChan:
			fmt.Println("\n[!] Teste interrompido pelo usuário.")
			break loop
		case <-testTimer.C:
			break loop
		case <-ticker.C:
			seq++
			nowNs := uint64(time.Now().UnixNano())
			binary.BigEndian.PutUint64(sendPayload[0:8], seq)
			binary.BigEndian.PutUint64(sendPayload[8:16], nowNs)
			binary.BigEndian.PutUint64(sendPayload[16:24], 0)

			_, err := conn.Write(sendPayload)
			if err == nil {
				mu.Lock()
				packetsSent++
				mu.Unlock()
			}
		}
	}

	// Aguarda 800ms para receber últimos pacotes em trânsito
	time.Sleep(800 * time.Millisecond)
	close(stopChan)

	totalDuration := time.Since(startTime)

	mu.Lock()
	defer mu.Unlock()

	printClientSummary(totalDuration, packetsSent, packetsRecv, outOfOrder, jitterRFC3550, samples, cfg.PacketSize)
}

func printClientSummary(elapsed time.Duration, sent, recv, outOfOrder uint64, jitter float64, samples []PacketStats, packetSize int) {
	fmt.Println("\n")
	fmt.Println("================================================================================")
	fmt.Println("                 RELATÓRIO DE QUALIDADE DA CONEXÃO P2P (UDP)                    ")
	fmt.Println("================================================================================")
	fmt.Printf(" Duração da Medição     : %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf(" Pacotes Enviados       : %d\n", sent)
	fmt.Printf(" Pacotes Recebidos      : %d\n", recv)

	var lossPercent float64
	lostCount := int64(sent) - int64(recv)
	if lostCount < 0 {
		lostCount = 0
	}
	if sent > 0 {
		lossPercent = (float64(lostCount) / float64(sent)) * 100.0
	}
	fmt.Printf(" Pacotes Perdidos       : %d (%.2f%%)\n", lostCount, lossPercent)
	fmt.Printf(" Pacotes Fora de Ordem  : %d\n", outOfOrder)

	totalBytes := recv * uint64(packetSize)
	kbps := (float64(totalBytes*8) / elapsed.Seconds()) / 1000.0
	fmt.Printf(" Throughput Médio Efetivo: %.2f Kbps (%.2f KB/s)\n", kbps, float64(totalBytes)/(elapsed.Seconds()*1024.0))

	if len(samples) > 0 {
		rtts := make([]time.Duration, len(samples))
		var sumRTT time.Duration
		for i, s := range samples {
			rtts[i] = s.RTT
			sumRTT += s.RTT
		}
		sort.Slice(rtts, func(i, j int) bool {
			return rtts[i] < rtts[j]
		})

		minRTT := rtts[0]
		maxRTT := rtts[len(rtts)-1]
		avgRTT := sumRTT / time.Duration(len(samples))
		p50 := rtts[len(rtts)*50/100]
		p95 := rtts[len(rtts)*95/100]
		p99 := rtts[len(rtts)*99/100]

		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Printf(" Latência RTT (Ida + Volta):\n")
		fmt.Printf("   Mínima: %-8v | Média: %-8v | Máxima: %-8v\n", minRTT.Round(time.Millisecond), avgRTT.Round(time.Millisecond), maxRTT.Round(time.Millisecond))
		fmt.Printf("   p50   : %-8v | p95  : %-8v | p99   : %-8v\n", p50.Round(time.Millisecond), p95.Round(time.Millisecond), p99.Round(time.Millisecond))
		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Printf(" Jitter Estatístico (RFC 3550) : %.2f ms\n", jitter)
		fmt.Println("--------------------------------------------------------------------------------")

		// Classificação de Qualidade para Suporte Remoto / WebRTC
		fmt.Print(" DIAGNÓSTICO DE QUALIDADE WEBRTC : ")
		if lossPercent <= 0.5 && avgRTT < 50*time.Millisecond && jitter < 15.0 {
			fmt.Println("EXCELENTE (Ideal para compartilhamento em 60fps e voz cristalina)")
		} else if lossPercent <= 2.0 && avgRTT < 120*time.Millisecond && jitter < 30.0 {
			fmt.Println("BOM (Experiência fluida sem atrasos perceptíveis)")
		} else if lossPercent <= 5.0 && avgRTT < 250*time.Millisecond && jitter < 50.0 {
			fmt.Println("REGULAR (Pode ocorrer leve congelamento momentâneo ou artefatos)")
		} else {
			fmt.Println("CRÍTICO / RUIM (Risco alto de desconexões, voz robotizada e tela borrada)")
		}
	} else {
		fmt.Println("\n[!] Nenhum pacote retornado pelo servidor. Verifique o firewall/porta UDP ou IP.")
	}

	fmt.Println("================================================================================")
}
