// Package voice 提供语音处理核心功能
// convert.go 提供音频格式转换功能
package voice

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// 支持的SILK采样率
var silkSupportedRates = []int{8000, 12000, 16000, 24000, 32000, 44100, 48000}

// ConvertOptions 定义音频转换选项
type ConvertOptions struct {
	// SampleRate 目标采样率
	SampleRate int
	// Channels 目标声道数(1=单声道, 2=立体声)
	Channels int
	// BitDepth 位深度(8, 16, 24, 32)
	BitDepth int
}

// DefaultConvertOptions 返回默认转换选项
func DefaultConvertOptions() ConvertOptions {
	return ConvertOptions{
		SampleRate: 16000,
		Channels:   1,
		BitDepth:   16,
	}
}

// Converter 音频格式转换器接口
type Converter interface {
	// Convert 执行格式转换
	Convert(input []byte, srcFormat, dstFormat AudioFormat, opts ConvertOptions) ([]byte, error)

	// ConvertFile 文件格式转换
	ConvertFile(srcPath, dstPath string, opts ConvertOptions) error
}

// AudioConverter 音频转换器实现
type AudioConverter struct{}

// NewAudioConverter 创建音频转换器
func NewAudioConverter() *AudioConverter {
	return &AudioConverter{}
}

// Convert 执行音频格式转换
func (c *AudioConverter) Convert(input []byte, srcFormat, dstFormat AudioFormat, opts ConvertOptions) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("输入音频数据为空")
	}

	if srcFormat == dstFormat {
		return input, nil
	}

	// 先解码为PCM
	pcmData, err := c.decodeToPCM(input, srcFormat, opts)
	if err != nil {
		return nil, fmt.Errorf("解码失败: %w", err)
	}

	// 再编码为目标格式
	output, err := c.encodeFromPCM(pcmData, dstFormat, opts)
	if err != nil {
		return nil, fmt.Errorf("编码失败: %w", err)
	}

	return output, nil
}

// decodeToPCM 将音频解码为PCM原始数据
func (c *AudioConverter) decodeToPCM(input []byte, format AudioFormat, opts ConvertOptions) ([]byte, error) {
	switch format {
	case FormatWAV:
		return c.wavToPCM(input)
	case FormatPCM:
		return input, nil
	case FormatMP3, FormatOGG, FormatAMR, FormatM4A:
		// 这些格式需要外部库支持，返回提示
		return nil, fmt.Errorf("格式 %s 需要外部库支持(如ffmpeg)", format)
	case FormatSILK:
		return c.silkToPCM(input, opts.SampleRate)
	default:
		return nil, fmt.Errorf("不支持的源格式: %s", format)
	}
}

// encodeFromPCM 将PCM数据编码为目标格式
func (c *AudioConverter) encodeFromPCM(pcm []byte, format AudioFormat, opts ConvertOptions) ([]byte, error) {
	switch format {
	case FormatWAV:
		return c.pcmToWAV(pcm, opts)
	case FormatPCM:
		return pcm, nil
	case FormatMP3, FormatOGG, FormatAMR, FormatM4A:
		return nil, fmt.Errorf("格式 %s 需要外部库支持(如ffmpeg)", format)
	case FormatSILK:
		return c.pcmToSILK(pcm, opts.SampleRate)
	default:
		return nil, fmt.Errorf("不支持的目标格式: %s", format)
	}
}

// WAVHeader WAV文件头结构
type WAVHeader struct {
	ChunkID       [4]byte // "RIFF"
	ChunkSize     uint32
	Format        [4]byte // "WAVE"
	Subchunk1ID   [4]byte // "fmt "
	Subchunk1Size uint32
	AudioFormat   uint16
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	Subchunk2ID   [4]byte // "data"
	Subchunk2Size uint32
}

// pcmToWAV 将PCM数据转换为WAV格式
func (c *AudioConverter) pcmToWAV(pcm []byte, opts ConvertOptions) ([]byte, error) {
	if len(pcm) == 0 {
		return nil, errors.New("PCM数据为空")
	}

	// 设置默认值
	if opts.SampleRate <= 0 {
		opts.SampleRate = 16000
	}
	if opts.Channels <= 0 {
		opts.Channels = 1
	}
	if opts.BitDepth <= 0 {
		opts.BitDepth = 16
	}

	header := WAVHeader{
		ChunkID:       [4]byte{'R', 'I', 'F', 'F'},
		Format:        [4]byte{'W', 'A', 'V', 'E'},
		Subchunk1ID:   [4]byte{'f', 'm', 't', ' '},
		Subchunk1Size: 16,
		AudioFormat:   1, // PCM
		NumChannels:   uint16(opts.Channels),
		SampleRate:    uint32(opts.SampleRate),
		BitsPerSample: uint16(opts.BitDepth),
		Subchunk2ID:   [4]byte{'d', 'a', 't', 'a'},
	}

	header.BlockAlign = header.NumChannels * header.BitsPerSample / 8
	header.ByteRate = header.SampleRate * uint32(header.BlockAlign)
	header.Subchunk2Size = uint32(len(pcm))
	header.ChunkSize = 36 + header.Subchunk2Size

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("写入WAV头失败: %w", err)
	}
	if _, err := buf.Write(pcm); err != nil {
		return nil, fmt.Errorf("写入PCM数据失败: %w", err)
	}

	return buf.Bytes(), nil
}

// wavToPCM 从WAV文件提取PCM数据
func (c *AudioConverter) wavToPCM(input []byte) ([]byte, error) {
	if len(input) < 44 {
		return nil, errors.New("无效的WAV文件: 文件过小")
	}

	// 验证WAV头
	if !bytes.Equal(input[0:4], []byte("RIFF")) || !bytes.Equal(input[8:12], []byte("WAVE")) {
		return nil, errors.New("无效的WAV文件格式")
	}

	// 查找data块
	dataOffset := 12
	for dataOffset < len(input)-8 {
		chunkID := string(input[dataOffset : dataOffset+4])
		chunkSize := binary.LittleEndian.Uint32(input[dataOffset+4 : dataOffset+8])
		if chunkID == "data" {
			dataOffset += 8
			if dataOffset+int(chunkSize) > len(input) {
				return nil, errors.New("WAV文件数据不完整")
			}
			return input[dataOffset : dataOffset+int(chunkSize)], nil
		}
		dataOffset += 8 + int(chunkSize)
	}

	return nil, errors.New("未找到WAV数据块")
}

// silkToPCM 将SILK格式转换为PCM
// 注意: 实际SILK解码需要外部库支持
func (c *AudioConverter) silkToPCM(input []byte, sampleRate int) ([]byte, error) {
	// SILK解码需要使用腾讯的silk库或类似实现
	// 这里提供接口框架，实际实现需要引入外部依赖
	return nil, errors.New("SILK解码需要外部库支持(如 github.com/wdvxdr1123/go-silk)")
}

// pcmToSILK 将PCM转换为SILK格式
// 注意: 实际SILK编码需要外部库支持
func (c *AudioConverter) pcmToSILK(pcm []byte, sampleRate int) ([]byte, error) {
	// SILK编码需要使用腾讯的silk库或类似实现
	return nil, errors.New("SILK编码需要外部库支持(如 github.com/wdvxdr1123/go-silk)")
}

// FindClosestSilkRate 找到最接近的支持的SILK采样率
func FindClosestSilkRate(sampleRate int) int {
	for _, rate := range silkSupportedRates {
		if rate == sampleRate {
			return rate
		}
	}

	closest := silkSupportedRates[0]
	minDiff := math.Abs(float64(sampleRate - closest))

	for _, rate := range silkSupportedRates {
		diff := math.Abs(float64(sampleRate - rate))
		if diff < minDiff {
			minDiff = diff
			closest = rate
		}
	}

	return closest
}

// GetWAVInfo 获取WAV文件信息
func GetWAVInfo(data []byte) (sampleRate, channels, bitsPerSample int, err error) {
	if len(data) < 44 {
		return 0, 0, 0, errors.New("无效的WAV文件")
	}

	header := WAVHeader{}
	reader := bytes.NewReader(data[0:44])
	if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
		return 0, 0, 0, fmt.Errorf("读取WAV头失败: %w", err)
	}

	return int(header.SampleRate), int(header.NumChannels), int(header.BitsPerSample), nil
}

// ResampleOptions 重采样选项
type ResampleOptions struct {
	SrcRate  int
	DstRate  int
	Channels int
}

// ResampleSimple 简单重采样(线性插值)
// 仅适用于简单场景，高质量重采样建议使用专业库
func ResampleSimple(pcm []byte, opts ResampleOptions) ([]byte, error) {
	if opts.SrcRate <= 0 || opts.DstRate <= 0 {
		return nil, errors.New("采样率必须大于0")
	}

	if opts.SrcRate == opts.DstRate {
		return pcm, nil
	}

	// 假设16位PCM
	sampleCount := len(pcm) / 2
	newSampleCount := int(float64(sampleCount) * float64(opts.DstRate) / float64(opts.SrcRate))

	result := make([]byte, newSampleCount*2)
	ratio := float64(sampleCount-1) / float64(newSampleCount-1)

	for i := 0; i < newSampleCount; i++ {
		srcPos := float64(i) * ratio
		srcIdx := int(srcPos)

		if srcIdx*2+3 < len(pcm) {
			// 线性插值
			frac := srcPos - float64(srcIdx)
			sample1 := int16(binary.LittleEndian.Uint16(pcm[srcIdx*2:]))
			sample2 := int16(binary.LittleEndian.Uint16(pcm[srcIdx*2+2:]))
			sample := sample1 + int16(float64(sample2-sample1)*frac)
			binary.LittleEndian.PutUint16(result[i*2:], uint16(sample))
		} else if srcIdx*2+1 < len(pcm) {
			binary.LittleEndian.PutUint16(result[i*2:], binary.LittleEndian.Uint16(pcm[srcIdx*2:]))
		}
	}

	return result, nil
}

// SplitAudio 分割音频数据
func SplitAudio(pcm []byte, maxSegmentBytes int) ([][]byte, error) {
	if len(pcm) <= maxSegmentBytes {
		return [][]byte{pcm}, nil
	}

	var segments [][]byte
	for offset := 0; offset < len(pcm); offset += maxSegmentBytes {
		end := offset + maxSegmentBytes
		if end > len(pcm) {
			end = len(pcm)
		}
		segments = append(segments, pcm[offset:end])
	}

	return segments, nil
}

// ReadPCMFromReader 从Reader读取PCM数据
func ReadPCMFromReader(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// WritePCMToWriter 将PCM数据写入Writer
func WritePCMToWriter(w io.Writer, pcm []byte) error {
	_, err := w.Write(pcm)
	return err
}

// ValidatePCMData 验证PCM数据有效性
func ValidatePCMData(pcm []byte, bitDepth int) error {
	if len(pcm) == 0 {
		return errors.New("PCM数据为空")
	}

	bytesPerSample := bitDepth / 8
	if bytesPerSample == 0 {
		bytesPerSample = 2 // 默认16位
	}

	if len(pcm)%bytesPerSample != 0 {
		return fmt.Errorf("PCM数据长度 %d 不是 %d 的倍数", len(pcm), bytesPerSample)
	}

	return nil
}
