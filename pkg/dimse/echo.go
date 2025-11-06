package dimse

import (
	"context"
	"fmt"
	"time"
)

// CEcho performs a C-ECHO operation (DICOM ping)
func (a *Association) CEcho(ctx context.Context) error {
	if !a.IsConnected() {
		if err := a.Connect(ctx); err != nil {
			return err
		}
	}

	a.UpdateLastUsed()

	// Build C-ECHO-RQ command
	command := a.buildCEchoRequest()

	// Send C-ECHO-RQ
	if err := a.sendCommand(command); err != nil {
		return fmt.Errorf("failed to send C-ECHO request: %w", err)
	}

	// Receive C-ECHO-RSP
	response, err := a.receiveCommand(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive C-ECHO response: %w", err)
	}

	// Check status
	status := a.getCommandStatus(response)
	if status != 0x0000 {
		return fmt.Errorf("C-ECHO failed with status: 0x%04x", status)
	}

	return nil
}

// buildCEchoRequest builds a C-ECHO-RQ command dataset
func (a *Association) buildCEchoRequest() []byte {
	// Simplified C-ECHO-RQ command
	// In production, build proper DICOM command dataset with:
	// - (0000,0002) Affected SOP Class UID
	// - (0000,0100) Command Field (C-ECHO-RQ = 0x0030)
	// - (0000,0110) Message ID
	// - (0000,0800) Command Data Set Type (0x0101 = null)

	command := []byte{
		0x04, 0x00, 0x00, 0x00, // P-DATA-TF PDU type
	}

	// TODO: Build actual DICOM command dataset

	return command
}

// sendCommand sends a DICOM command via P-DATA-TF PDU
func (a *Association) sendCommand(command []byte) error {
	if err := a.conn.SetWriteDeadline(time.Now().Add(a.timeout)); err != nil {
		return err
	}

	_, err := a.conn.Write(command)
	return err
}

// receiveCommand receives a DICOM command response
func (a *Association) receiveCommand(ctx context.Context) ([]byte, error) {
	if err := a.conn.SetReadDeadline(time.Now().Add(a.timeout)); err != nil {
		return nil, err
	}

	// Read PDU header
	header := make([]byte, 6)
	_, err := a.conn.Read(header)
	if err != nil {
		return nil, err
	}

	// Get PDU length
	length := uint32(header[2])<<24 | uint32(header[3])<<16 | uint32(header[4])<<8 | uint32(header[5])

	// Read PDU data
	data := make([]byte, length)
	_, err = a.conn.Read(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// getCommandStatus extracts status from command response
func (a *Association) getCommandStatus(response []byte) uint16 {
	// TODO: Parse DICOM command dataset and extract (0000,0900) Status
	// For now, return success
	return 0x0000
}
