package segmentation

import (
	pb_segmentation "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/segmentation"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

// Splits a buffer into multiple chunks of size max n
func splitBuffer(buf []byte, n int) [][]byte {
	var chunks [][]byte
	for i := 0; i < len(buf); i += n {
		end := i + n
		if end > len(buf) {
			end = len(buf)
		}
		chunks = append(chunks, buf[i:end])
	}
	return chunks
}

// Add segment id, packet id and segment length information to a buffer
func addSegmentInformation(buf []byte, packetId int64, segmentId int64, totalSegments int64) []byte {
	seg := pb_segmentation.Segment{
		PacketId:      packetId,
		SegmentId:     segmentId,
		TotalSegments: totalSegments,
		Data:          buf,
	}
	segmentBytes, err := proto.Marshal(&seg)
	if err != nil {
		log.Err(err).Msg("Failed to marshal segment")
		return nil
	}

	return segmentBytes
}

// Splits buffer into standardized protobuf segments
func SegmentBuffer(buf []byte, packetId int64) [][]byte {
	// Split the buffer into chunks
	chunks := splitBuffer(buf, 32192)
	// Add segment information to each chunk and send it
	for i, chunk := range chunks {
		chunks[i] = addSegmentInformation(chunk, packetId, int64(i), int64((len(chunks))))
	}

	return chunks
}
