package dnsengine

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// Codes de réponse DNS utiles ici.
const (
	RcodeNoError  = 0
	RcodeServFail = 2
	RcodeNXDomain = 3
)

// Answer résume une réponse DNS.
type Answer struct {
	Rcode   int
	Answers int
}

// buildQuery fabrique une requête DNS de type A, récursion demandée.
func buildQuery(id uint16, name string) ([]byte, error) {
	name = strings.TrimSuffix(name, ".")
	if name == "" {
		return nil, errors.New("nom de domaine vide")
	}
	msg := make([]byte, 12, 64)
	binary.BigEndian.PutUint16(msg[0:2], id)
	binary.BigEndian.PutUint16(msg[2:4], 0x0100) // RD
	binary.BigEndian.PutUint16(msg[4:6], 1)      // QDCOUNT
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 {
			return nil, fmt.Errorf("label invalide dans %q", name)
		}
		msg = append(msg, byte(len(label)))
		msg = append(msg, label...)
	}
	msg = append(msg, 0)    // fin du nom
	msg = append(msg, 0, 1) // QTYPE A
	msg = append(msg, 0, 1) // QCLASS IN
	return msg, nil
}

// parseHeader lit l'en-tête d'une réponse et vérifie qu'elle correspond à la requête.
func parseHeader(id uint16, resp []byte) (Answer, error) {
	if len(resp) < 12 {
		return Answer{}, errors.New("réponse DNS trop courte")
	}
	if binary.BigEndian.Uint16(resp[0:2]) != id {
		return Answer{}, errors.New("identifiant de réponse inattendu")
	}
	if resp[2]&0x80 == 0 {
		return Answer{}, errors.New("le paquet reçu n'est pas une réponse")
	}
	return Answer{
		Rcode:   int(resp[3] & 0x0F),
		Answers: int(binary.BigEndian.Uint16(resp[6:8])),
	}, nil
}

// Query envoie une requête A au serveur donné (ex. "127.0.0.1:53").
func Query(server, name string, timeout time.Duration) (Answer, error) {
	var idb [2]byte
	if _, err := rand.Read(idb[:]); err != nil {
		return Answer{}, err
	}
	id := binary.BigEndian.Uint16(idb[:])

	q, err := buildQuery(id, name)
	if err != nil {
		return Answer{}, err
	}

	conn, err := net.DialTimeout("udp", server, timeout)
	if err != nil {
		return Answer{}, fmt.Errorf("connexion à %s impossible : %w", server, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	if _, err := conn.Write(q); err != nil {
		return Answer{}, fmt.Errorf("envoi de la requête : %w", err)
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return Answer{}, fmt.Errorf("pas de réponse de %s : %w", server, err)
	}
	return parseHeader(id, buf[:n])
}
