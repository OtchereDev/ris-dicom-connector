package dimse

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// Association represents a DICOM association
type Association struct {
	conn         net.Conn
	callingAET   string
	calledAET    string
	host         string
	port         int
	maxPDULength uint32
	timeout      time.Duration
	mu           sync.Mutex
	isConnected  bool
	lastUsed     time.Time
}

// AssociationConfig holds configuration for DICOM associations
type AssociationConfig struct {
	Host         string
	Port         int
	CallingAET   string
	CalledAET    string
	Timeout      time.Duration
	MaxPDULength uint32
}

// NewAssociation creates a new DICOM association
func NewAssociation(config AssociationConfig) *Association {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxPDULength == 0 {
		config.MaxPDULength = 16384 // 16KB default
	}

	return &Association{
		callingAET:   config.CallingAET,
		calledAET:    config.CalledAET,
		host:         config.Host,
		port:         config.Port,
		maxPDULength: config.MaxPDULength,
		timeout:      config.Timeout,
	}
}

// Connect establishes a DICOM association
func (a *Association) Connect(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isConnected {
		return nil
	}

	// Create TCP connection
	addr := fmt.Sprintf("%s:%d", a.host, a.port)
	dialer := &net.Dialer{
		Timeout: a.timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to PACS: %w", err)
	}

	a.conn = conn
	a.isConnected = true
	a.lastUsed = time.Now()

	// Send A-ASSOCIATE-RQ
	if err := a.sendAssociateRequest(ctx); err != nil {
		a.Close()
		return fmt.Errorf("failed to send associate request: %w", err)
	}

	// Receive A-ASSOCIATE-AC
	if err := a.receiveAssociateResponse(ctx); err != nil {
		a.Close()
		return fmt.Errorf("failed to receive associate response: %w", err)
	}

	return nil
}

// Close closes the DICOM association
func (a *Association) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isConnected {
		return nil
	}

	// Send A-RELEASE-RQ
	if err := a.sendReleaseRequest(); err != nil {
		// Log but continue to close connection
		fmt.Printf("Error sending release request: %v\n", err)
	}

	a.isConnected = false
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

// IsConnected checks if the association is still active
func (a *Association) IsConnected() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.isConnected
}

// UpdateLastUsed updates the last used timestamp
func (a *Association) UpdateLastUsed() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastUsed = time.Now()
}

// GetLastUsed returns the last used timestamp
func (a *Association) GetLastUsed() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastUsed
}

// sendAssociateRequest sends A-ASSOCIATE-RQ PDU
func (a *Association) sendAssociateRequest(ctx context.Context) error {
	// Build A-ASSOCIATE-RQ PDU
	pdu := a.buildAssociateRequestPDU()

	// Set write deadline
	if err := a.conn.SetWriteDeadline(time.Now().Add(a.timeout)); err != nil {
		return err
	}

	// Send PDU
	_, err := a.conn.Write(pdu)
	return err
}

// receiveAssociateResponse receives A-ASSOCIATE-AC PDU
func (a *Association) receiveAssociateResponse(ctx context.Context) error {
	// Set read deadline
	if err := a.conn.SetReadDeadline(time.Now().Add(a.timeout)); err != nil {
		return err
	}

	// Read PDU header (first 6 bytes)
	header := make([]byte, 6)
	_, err := a.conn.Read(header)
	if err != nil {
		return fmt.Errorf("failed to read PDU header: %w", err)
	}

	// Verify PDU type (0x02 = A-ASSOCIATE-AC)
	if header[0] != 0x02 {
		return fmt.Errorf("unexpected PDU type: 0x%02x", header[0])
	}

	// Read PDU length (bytes 2-5, big endian)
	length := uint32(header[2])<<24 | uint32(header[3])<<16 | uint32(header[4])<<8 | uint32(header[5])

	// Read PDU data
	data := make([]byte, length)
	_, err = a.conn.Read(data)
	if err != nil {
		return fmt.Errorf("failed to read PDU data: %w", err)
	}

	// Parse and validate A-ASSOCIATE-AC
	// (Simplified - in production, parse all presentation contexts)

	return nil
}

// sendReleaseRequest sends A-RELEASE-RQ PDU
func (a *Association) sendReleaseRequest() error {
	// A-RELEASE-RQ PDU
	pdu := []byte{
		0x05,                   // PDU type: A-RELEASE-RQ
		0x00,                   // Reserved
		0x00, 0x00, 0x00, 0x04, // PDU length: 4
		0x00, 0x00, 0x00, 0x00, // Reserved
	}

	if err := a.conn.SetWriteDeadline(time.Now().Add(a.timeout)); err != nil {
		return err
	}

	_, err := a.conn.Write(pdu)
	return err
}

// buildAssociateRequestPDU builds A-ASSOCIATE-RQ PDU
func (a *Association) buildAssociateRequestPDU() []byte {
	// Simplified A-ASSOCIATE-RQ PDU
	// In production, this should include:
	// - Application Context
	// - Presentation Contexts (for each supported SOP class)
	// - User Information

	pdu := []byte{0x01, 0x00} // PDU type: A-ASSOCIATE-RQ, Reserved

	// Protocol version (bytes 2-3)
	pdu = append(pdu, 0x00, 0x01)

	// Reserved (bytes 4-5)
	pdu = append(pdu, 0x00, 0x00)

	// Called AE Title (16 bytes, padded with spaces)
	calledAET := padAET(a.calledAET)
	pdu = append(pdu, calledAET...)

	// Calling AE Title (16 bytes, padded with spaces)
	callingAET := padAET(a.callingAET)
	pdu = append(pdu, callingAET...)

	// Reserved (32 bytes)
	reserved := make([]byte, 32)
	pdu = append(pdu, reserved...)

	// Application Context Item
	pdu = append(pdu, a.buildApplicationContext()...)

	// Presentation Context Items
	pdu = append(pdu, a.buildPresentationContexts()...)

	// User Information Item
	pdu = append(pdu, a.buildUserInformation()...)

	// Update PDU length (bytes 2-5 of header)
	length := uint32(len(pdu) - 6)
	pdu[2] = byte(length >> 24)
	pdu[3] = byte(length >> 16)
	pdu[4] = byte(length >> 8)
	pdu[5] = byte(length)

	return pdu
}

// buildApplicationContext builds Application Context item
func (a *Association) buildApplicationContext() []byte {
	// Application Context Name: 1.2.840.10008.3.1.1.1 (DICOM Application Context)
	uid := "1.2.840.10008.3.1.1.1"

	item := []byte{0x10, 0x00} // Item type: Application Context

	// Length (2 bytes)
	length := uint16(len(uid))
	item = append(item, byte(length>>8), byte(length))

	// UID
	item = append(item, []byte(uid)...)

	return item
}

// buildPresentationContexts builds Presentation Context items
func (a *Association) buildPresentationContexts() []byte {
	var contexts []byte

	// Add common SOP classes
	sopClasses := []string{
		"1.2.840.10008.5.1.4.1.2.1.1", // Patient Root Query/Retrieve - FIND
		"1.2.840.10008.5.1.4.1.2.1.2", // Patient Root Query/Retrieve - MOVE
		"1.2.840.10008.5.1.4.1.2.1.3", // Patient Root Query/Retrieve - GET
		"1.2.840.10008.5.1.4.1.2.2.1", // Study Root Query/Retrieve - FIND
		"1.2.840.10008.5.1.4.1.2.2.2", // Study Root Query/Retrieve - MOVE
		"1.2.840.10008.5.1.4.1.2.2.3", // Study Root Query/Retrieve - GET
		"1.2.840.10008.1.1",           // Verification SOP Class (C-ECHO)
	}

	presentationContextID := byte(1)
	for _, sopClass := range sopClasses {
		ctx := a.buildPresentationContext(presentationContextID, sopClass)
		contexts = append(contexts, ctx...)
		presentationContextID += 2 // Must be odd numbers
	}

	return contexts
}

// buildPresentationContext builds a single Presentation Context item
func (a *Association) buildPresentationContext(id byte, sopClass string) []byte {
	item := []byte{0x20, 0x00} // Item type: Presentation Context

	// Placeholder for length (will update later)
	lengthPos := len(item)
	item = append(item, 0x00, 0x00)

	// Presentation Context ID
	item = append(item, id)

	// Reserved (3 bytes)
	item = append(item, 0x00, 0x00, 0x00)

	// Abstract Syntax Sub-item
	abstractSyntax := []byte{0x30, 0x00} // Item type: Abstract Syntax
	abstractSyntax = append(abstractSyntax, byte(len(sopClass)>>8), byte(len(sopClass)))
	abstractSyntax = append(abstractSyntax, []byte(sopClass)...)
	item = append(item, abstractSyntax...)

	// Transfer Syntax Sub-items
	transferSyntaxes := []string{
		"1.2.840.10008.1.2",   // Implicit VR Little Endian
		"1.2.840.10008.1.2.1", // Explicit VR Little Endian
		"1.2.840.10008.1.2.2", // Explicit VR Big Endian
	}

	for _, ts := range transferSyntaxes {
		transferSyntax := []byte{0x40, 0x00} // Item type: Transfer Syntax
		transferSyntax = append(transferSyntax, byte(len(ts)>>8), byte(len(ts)))
		transferSyntax = append(transferSyntax, []byte(ts)...)
		item = append(item, transferSyntax...)
	}

	// Update length
	length := uint16(len(item) - 4)
	item[lengthPos] = byte(length >> 8)
	item[lengthPos+1] = byte(length)

	return item
}

// buildUserInformation builds User Information item
func (a *Association) buildUserInformation() []byte {
	item := []byte{0x50, 0x00} // Item type: User Information

	// Placeholder for length (will update later)
	lengthPos := len(item)
	item = append(item, 0x00, 0x00)

	// Maximum Length Sub-item
	maxLength := []byte{
		0x51, 0x00, // Item type: Maximum Length
		0x00, 0x04, // Length: 4
	}
	maxLength = append(maxLength,
		byte(a.maxPDULength>>24),
		byte(a.maxPDULength>>16),
		byte(a.maxPDULength>>8),
		byte(a.maxPDULength),
	)
	item = append(item, maxLength...)

	// Implementation Class UID Sub-item
	implClassUID := "1.2.826.0.1.3680043.9.7433.1.1" // Our implementation UID
	implClass := []byte{0x52, 0x00}                  // Item type: Implementation Class UID
	implClass = append(implClass, byte(len(implClassUID)>>8), byte(len(implClassUID)))
	implClass = append(implClass, []byte(implClassUID)...)
	item = append(item, implClass...)

	// Implementation Version Name Sub-item
	implVersion := "DICOM_CONNECTOR_V1"
	implVer := []byte{0x55, 0x00} // Item type: Implementation Version Name
	implVer = append(implVer, byte(len(implVersion)>>8), byte(len(implVersion)))
	implVer = append(implVer, []byte(implVersion)...)
	item = append(item, implVer...)

	// Update length
	length := uint16(len(item) - 4)
	item[lengthPos] = byte(length >> 8)
	item[lengthPos+1] = byte(length)

	return item
}

// padAET pads AE Title to 16 bytes with spaces
func padAET(aet string) []byte {
	result := make([]byte, 16)
	copy(result, []byte(aet))
	for i := len(aet); i < 16; i++ {
		result[i] = ' '
	}
	return result
}
