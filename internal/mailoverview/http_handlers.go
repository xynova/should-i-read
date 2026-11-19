package mailoverview

import "net/http"

// Handler types split overview HTTP verbs off IntentPageServer.

type pageHTTP struct{ *IntentPageServer }

type intentHTTP struct{ *IntentPageServer }

type cartHTTP struct{ *IntentPageServer }

type cartJobHTTP struct{ *IntentPageServer }

type ackHTTP struct{ *IntentPageServer }

type keepHTTP struct{ *IntentPageServer }

type actionedHTTP struct{ *IntentPageServer }

type previewHTTP struct{ *IntentPageServer }

type syncHTTP struct{ *IntentPageServer }

type eventsHTTP struct{ *IntentPageServer }

func (s *IntentPageServer) mountHandlers(mux *http.ServeMux) {
	page := &pageHTTP{s}
	intent := &intentHTTP{s}
	cart := &cartHTTP{s}
	cartJobs := &cartJobHTTP{s}
	ack := &ackHTTP{s}
	keep := &keepHTTP{s}
	actioned := &actionedHTTP{s}
	preview := &previewHTTP{s}
	syncH := &syncHTTP{s}
	events := &eventsHTTP{s}
	mux.HandleFunc("/", page.servePage)
	mux.HandleFunc("/intent", intent.serveIntent)
	mux.HandleFunc("/intent/detail", intent.serveIntentDetail)
	mux.HandleFunc("/cart", cart.serveCart)
	mux.HandleFunc("/cart/checkout", cartJobs.serveCartCheckout)
	mux.HandleFunc("/cart/jobs", cartJobs.serveCartJobs)
	mux.HandleFunc("/cart/jobs/", cartJobs.serveCartJobs)
	mux.HandleFunc("/ack", ack.serveAck)
	mux.HandleFunc("/ack/checkout", ack.serveAckCheckout)
	mux.HandleFunc("/ack/jobs", ack.serveAckJobs)
	mux.HandleFunc("/ack/jobs/", ack.serveAckJobs)
	mux.HandleFunc("/actioned", actioned.serveActioned)
	mux.HandleFunc("/keep", keep.serveKeep)
	mux.HandleFunc("/preview", preview.servePreview)
	mux.HandleFunc("/sync", syncH.serveSync)
	mux.HandleFunc("/events", events.serveEvents)
}
