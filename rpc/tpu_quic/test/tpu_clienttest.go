package main

import (
	"fmt"
	"github.com/desperatee/solana-go"
	"github.com/desperatee/solana-go/programs/system"
	"github.com/desperatee/solana-go/rpc"
	"github.com/desperatee/solana-go/rpc/tpu"
	"github.com/desperatee/solana-go/rpc/tpu_quic"
	"runtime"
	"time"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	rpcClient, err := rpc.New("https://mainnet-beta.solflare.network/")
	fmt.Println(err)
	//tpuClient, err := tpu.New(rpcClient, "wss://solana-mainnet.phantom.tech", tpu.TPUClientConfig{FanoutSlots: tpu.MAX_FANOUT_SLOTS})
	//fmt.Println(err)
	tpuClient, err := tpu_quic.New(rpcClient, "wss://mainnet-beta.solflare.network", tpu_quic.TPUClientConfig{FanoutSlots: tpu.MAX_FANOUT_SLOTS})
	fmt.Println(err)
	recentBlockhash, err := rpcClient.GetLatestBlockhash(rpc.CommitmentConfirmed)
	fmt.Println(err)

	wallet, _ := solana.WalletFromPrivateKeyBase58("4mWk5iNPveY8tVqTvAeLnkEWToR9DRNSoFHG5vLQR5PVWR2xXjzPm4oymVoQY2twzhdHzm7FTv9jMidAYHuLKQVp")
	transaction, _ := solana.NewTransaction(
		[]solana.Instruction{system.NewTransferInstruction(1, wallet.PublicKey(), wallet.PublicKey()).Build()},
		recentBlockhash.Value.Blockhash, solana.TransactionPayer(wallet.PublicKey()))
	message, _ := transaction.Message.MarshalBinary()
	signature, _ := wallet.PrivateKey.Sign(message)
	transaction.Signatures = append(transaction.Signatures, signature)
	raw, _ := transaction.MarshalBinary()
	//for {
	//	time.Sleep(250 * time.Millisecond)
	//	start := time.Now()
	//	err := tpuClient.SendRawTransactionSameConn(raw, 1)
	//	end := time.Since(start)
	//	fmt.Printf("[%v] %v (%v) %v\n", time.Now(), transaction.Signatures[0], err, end)
	//}
	channel := make(chan int)
	curr := time.Now()
	for i := 0; i < 16; i++ {
		go func() {
			<-channel
			for {
				start := time.Now()
				err := tpuClient.SendRawTransactionSameConn(raw, 1)
				end := time.Since(start)
				fmt.Printf("[%v] %v (%v) %v\n", time.Now(), transaction.Signatures[0], err, end)
			}
			channel <- 1
		}()
	}
	time.Sleep(1 * time.Second)
	for i := 0; i < 16; i++ {
		channel <- 1
	}
	done := 0
	for {
		if done == 16 {
			break
		}
		<-channel
		done++
	}
	fmt.Println(time.Since(curr))
	blabla := make(chan string)
	for {
		<-blabla
	}
}
