package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kushagra-gupta01/Cryto_Marketplace/orderbook"
	"github.com/labstack/echo/v4"
)

const (
	MarketOrder OrderType = "MARKET"
	LimitOrder OrderType = "LIMIT"
	
	MarketETH Market = "ETH"
	
	exchangePrivateKey = "43ca4cbeb5cb457ae4f468bc157d548741f1a96edf103c73776ba04ffe3e5ccc"
)
type(
	OrderType string
	Market string
	
	MatchedOrder struct{
		Price float64
		Size 	float64
		ID		int64
	}

	PlaceOrderRequest struct{
		UserID 	int64
		Type 		OrderType //market or limit
		Bid			bool
		Size 		float64
		Price 	float64
		Market 	Market
	}

	Order struct{
		UserID		int64
		ID 				int64
		Price 		float64
		Size 			float64
		Bid				bool
		TimeStamp int64
	}

	OrderBookData struct{
		TotalBidVolume float64
		TotalAskVolume float64
		Asks					 []*Order
		Bids					 []*Order
	}
)

func main(){
	e := echo.New()
	client,err := ethclient.Dial("HTTP://127.0.0.1:7545")
	if err!=nil{
		log.Fatal(err)
	}

	ex,err := NewExchange(exchangePrivateKey,client)
	if err!=nil{
		log.Fatal(err)
	}

	buyerAddressStr := "0xb06524e5b48FfaE36b9bd3916CaaC153d68B31BB"
	buyerBalance,err := client.BalanceAt(context.Background(),common.HexToAddress(buyerAddressStr),nil)
	if err !=nil{
		log.Fatal(err)
	}
	fmt.Println("buyer: ",buyerBalance)

	sellerAddressStr := "0xa786fD6453119Bf26e0276449F61dBBB0534d5c1"
	sellerBalance,err := client.BalanceAt(context.Background(),common.HexToAddress(sellerAddressStr),nil)
	if err !=nil{
		log.Fatal(err)
	}
	fmt.Println("seller: ",sellerBalance)

	pkstr1 := "54f306b738dbf41f7312401c24e511d68bd97e6a108a58ac3a76e163c6ed3be1"
	user1 := NewUser(pkstr1,1)
	ex.Users[user1.ID] = user1

	pkstr2 :="fc652f1c56c9c3d7e5a6fc9f6d59180fcc66e2ced701e9f37e2e53c8265ce46c"
	user2 := NewUser(pkstr2,2)
	ex.Users[user2.ID] = user2

	e.GET("/book/:market",ex.handleGetBook)
	e.POST("/order",ex.handlePlaceOrder)
	e.DELETE("/order/:id",ex.cancelOrder)

	e.Start(":3000")
}

type User struct{
	ID 					int64
	PrivateKey	*ecdsa.PrivateKey
}

func NewUser(privKey string,id int64)*User{
	pk,err := crypto.HexToECDSA(privKey)
	if err!=nil{
		panic(err)
	}
	return &User{
		ID: id,
		PrivateKey: pk,
	}
}

type Exchange struct{
	Client			*ethclient.Client
	Users 			map[int64]*User
	orders 			map[int64]int64
	PrivateKey *ecdsa.PrivateKey
	orderbooks map[Market]*orderbook.OrderBook
}


func NewExchange(privateKey string,client *ethclient.Client) (*Exchange,error){
	orderbooks := make(map[Market]*orderbook.OrderBook)
	orderbooks[MarketETH] = orderbook.NewOrderBook()
	
	pk,err :=crypto.HexToECDSA(privateKey)
	if err !=nil{
		return nil,err
	}

	return &Exchange{
		Client:			client,
		Users: 			make(map[int64]*User),
		orders: 		make(map[int64]int64),
		PrivateKey: pk,
		orderbooks: orderbooks,
	},nil
}

func (ex* Exchange) cancelOrder(c echo.Context) error{
	idStr := c.Param("id")
	id ,_ := strconv.Atoi(idStr)

	ob := ex.orderbooks[MarketETH]
	order := ob.Orders[int64(id)]
	ob.CancelOrder(order)

	return c.JSON(http.StatusOK,map[string]any{"msg":"order deleted"})
}

func(ex *Exchange) handleGetBook(c echo.Context)error{
	market := Market(c.Param("market"))
	ob,ok := ex.orderbooks[market] 
	if !ok{
		return c.JSON(http.StatusBadRequest,map[string]any{"msg":"Orderbook not found"})
	}

	orderbookData := OrderBookData{
		TotalBidVolume: ob.BidTotalVolume(),
		TotalAskVolume: ob.AskTotalVolume(),
		Asks: 					[]*Order{},
		Bids: 					[]*Order{},
	}	

	for _,limit := range ob.Asks(){
		for _,order := range limit.Orders{
			o := Order{
				UserID: 		order.UserID,
				ID: 				order.ID,		
				Price: 			limit.Price,
				Size: 			order.Size,
				Bid: 				order.Bid,
				TimeStamp: 	order.TimeStamp,
			}
			orderbookData.Asks = append(orderbookData.Asks,&o)
		}
	}

	for _,limit := range ob.Bids(){
		for _,order := range limit.Orders{
			o := Order{
				UserID: 		order.UserID,	
				ID: 				order.ID,
				Price: 			limit.Price,
				Size:				order.Size,
				Bid: 				order.Bid,
				TimeStamp: 	order.TimeStamp,
			}
			orderbookData.Bids = append(orderbookData.Bids,&o)
		}
	}

	return c.JSON(http.StatusOK,orderbookData)
}

func (ex *Exchange) handlePlaceMarketOrder(market Market,order *orderbook.Order)([]orderbook.Match,[]*MatchedOrder){
	ob := ex.orderbooks[market]
	matches := ob.PlaceMarketOrder(order)
	matchedOrders := make([]*MatchedOrder,len(matches))

	isBid := false
	if order.Bid{
		isBid = true
	}

	for i := 0; i < len(matchedOrders); i++ {
		id:= matches[i].Bid.ID
		if isBid{
			id = matches[i].Ask.ID
		}
		matchedOrders[i] = &MatchedOrder{
			ID: 		id,
			Size: 	matches[i].SizeFilled,
			Price: 	matches[i].Price,
		}
	}
	return matches,matchedOrders
}

func (ex *Exchange) handlePlaceLimitOrder(market Market,price float64,order *orderbook.Order)error{
	ob := ex.orderbooks[market]
	ob.PlaceLimitOrder(price,order)

	return nil
}

func(ex *Exchange) handlePlaceOrder(c echo.Context) error{
	var placeOrderData PlaceOrderRequest

	if err:= json.NewDecoder(c.Request().Body).Decode(&placeOrderData);err!=nil{
		return err
	}

	market:= Market(placeOrderData.Market)
	order := orderbook.NewOrder(placeOrderData.Bid,placeOrderData.Size,placeOrderData.UserID)

	if placeOrderData.Type == LimitOrder{
		if err := ex.handlePlaceLimitOrder(market,placeOrderData.Price,order);err!=nil{
			return err
		}
		return c.JSON(http.StatusOK,map[string]any{"msg":"limit order placed"})
	}

	if placeOrderData.Type == MarketOrder{
		matches,matchedOrders :=ex.handlePlaceMarketOrder(market,order)
		if err := ex.handleMatches(matches);err!=nil{
			return err
		}

		return c.JSON(http.StatusOK,map[string]any{"matches": matchedOrders})
	}
	return nil
}

func (ex *Exchange) handleMatches(matches []orderbook.Match) error{
	for _,match := range matches{
		fromUser,ok :=  ex.Users[match.Ask.UserID]
		if !ok{
			return fmt.Errorf("User not found: %d",match.Ask.UserID)
		}

		toUser,ok :=  ex.Users[match.Bid.UserID]
		if !ok{
			return fmt.Errorf("User not found: %d",match.Bid.UserID)
		}
		toAddress := crypto.PubkeyToAddress(toUser.PrivateKey.PublicKey)
	
	//this is only used for exchange fees
	// exchangePubKey := ex.PrivateKey.Public()
	// publicKeyECDSA,ok := exchangePubKey.(*ecdsa.PublicKey)
	// if !ok{
	// 	return fmt.Errorf("error casting public key to ECDSA")
	// }

	amount := big.NewInt(int64(match.SizeFilled))
	transferETH(ex.Client,fromUser.PrivateKey,toAddress,amount)
	}

	return nil
}