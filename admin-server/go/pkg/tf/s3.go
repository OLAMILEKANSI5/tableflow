package tf

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"io"
	"math"
	"os"
	"time"
)

type S3Handler struct {
	Session       *session.Session
	BucketUploads string
	BucketImports string
}

func (h S3Handler) DownloadFileToDisk(key, bucket string, file *os.File) error {
	Log.Debugw("Downloading file from S3 to disk", "key", key, "bucket", bucket)
	downloader := s3manager.NewDownloader(h.Session)
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	_, err := downloader.Download(file, input)
	if err == nil {
		Log.Debugw("File download to disk complete", "key", key, "bucket", bucket)
	}
	return err
}

func (h S3Handler) DownloadFile(key, bucket string) (*s3.GetObjectOutput, error) {
	Log.Debugw("Downloading file", "key", key, "bucket", bucket)
	results, err := s3.New(h.Session).GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	Log.Debugw("File download complete", "key", key, "bucket", bucket)
	return results, nil
}

func (h S3Handler) WaitForUploadCompletion(key, bucket string) error {
	waiterInput := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	waiterDelay := func(delay time.Duration) request.WaiterDelay {
		return func(attempt int) time.Duration {
			// Can be modified to adjust delay based on the attempt number
			return delay
		}
	}
	err := s3.New(h.Session).WaitUntilObjectExistsWithContext(aws.BackgroundContext(), waiterInput,
		request.WithWaiterMaxAttempts(30),
		request.WithWaiterDelay(waiterDelay(5*time.Second)),
	)
	if err != nil {
		Log.Errorw("Exceeded maximum attempts waiting for file upload to complete", "error", err, "key", key)
	}
	return err
}

func (h S3Handler) UploadFile(key, contentType, bucket string, file *os.File) error {
	Log.Debugw("Uploading file to S3", "key", key, "bucket", bucket)
	uploader := s3manager.NewUploader(h.Session, func(u *s3manager.Uploader) {
		u.PartSize = 64 * 1024 * 1024 // 64MB
	})
	_, err := uploader.Upload(&s3manager.UploadInput{
		Body:               file,
		Bucket:             aws.String(bucket),
		Key:                aws.String(key),
		ContentDisposition: aws.String("attachment"),
		ContentType:        aws.String(contentType),
	})
	Log.Debugw("File upload to S3 complete", "key", key)
	return err
}

func (h S3Handler) UploadFile_DO_NOT_USE(key, contentType, bucket string, file *os.File) error {
	_, err := s3.New(h.Session).PutObject(&s3.PutObjectInput{
		Bucket:             aws.String(bucket),
		Key:                aws.String(key),
		Body:               file,
		ContentDisposition: aws.String("attachment"),
		ContentType:        aws.String(contentType),
	})
	return err
}

func (h S3Handler) UploadFileMultipart_DO_NOT_USE(key, contentType, bucket string, file *os.File) error {
	Log.Debugw("Uploading file to S3", "key", key, "bucket", bucket)
	svc := s3.New(h.Session)
	createResp, err := svc.CreateMultipartUpload(&s3.CreateMultipartUploadInput{
		Bucket:             aws.String(bucket),
		Key:                aws.String(key),
		ContentDisposition: aws.String("attachment"),
		ContentType:        aws.String(contentType),
	})
	if err != nil {
		Log.Errorw("Failed to create multipart upload", "error", err, "key", key)
		return err
	}
	partSize := int64(64 * 1024 * 1024) // 64MB
	fileInfo, _ := file.Stat()
	partCount := int(fileInfo.Size()/partSize) + 1
	var completedParts []*s3.CompletedPart

	for i := 0; i < partCount; i++ {
		partNumber := i + 1
		offset := partSize * int64(i)
		_, err = file.Seek(offset, 0)
		if err != nil {
			Log.Errorw("Failed to seek in file during multipart upload", "error", err, "key", key)
			return err
		}
		size := int(math.Min(float64(partSize), float64(fileInfo.Size()-offset)))
		buffer := make([]byte, size)
		_, err = file.Read(buffer)
		if err != nil {
			Log.Errorw("Failed to read part during multipart upload", "error", err, "key", key)
			return err
		}
		completedPart, err := uploadPart(svc, createResp, buffer, size, partNumber, partCount)
		if err != nil {
			Log.Errorw("Failed to upload part during multipart upload", "error", err, "key", key)
			err = abortMultipartUpload(svc, createResp)
			if err != nil {
				Log.Errorw("Failed to abort multipart upload", "error", err, "key", key)
			}
			return err
		}
		completedParts = append(completedParts, completedPart)
	}
	_, err = svc.CompleteMultipartUpload(&s3.CompleteMultipartUploadInput{
		Bucket:   createResp.Bucket,
		Key:      createResp.Key,
		UploadId: createResp.UploadId,
		MultipartUpload: &s3.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		Log.Errorw("Failed to complete multipart upload", "error", err, "key", key)
		return err
	}
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		Log.Errorw("Error resetting file reader", "error", err, "key", key)
	}
	Log.Debugw("Multipart upload to S3 complete", "key", createResp.Key)
	return nil
}

func uploadPart(svc *s3.S3, resp *s3.CreateMultipartUploadOutput, buffer []byte, size, partNumber, partCount int) (*s3.CompletedPart, error) {
	tryNum := 1
	maxRetries := 3
	partInput := &s3.UploadPartInput{
		Body:          aws.ReadSeekCloser(bytes.NewReader(buffer)),
		Bucket:        resp.Bucket,
		Key:           resp.Key,
		UploadId:      resp.UploadId,
		PartNumber:    aws.Int64(int64(partNumber)),
		ContentLength: aws.Int64(int64(size)),
	}
	var uploadErr error
	for tryNum <= maxRetries {
		uploadResult, err := svc.UploadPart(partInput)
		if err == nil {
			Log.Debugw("Part upload successful", "key", resp.Key, "part", partNumber, "part_count", partCount)
			return &s3.CompletedPart{
				ETag:       uploadResult.ETag,
				PartNumber: aws.Int64(int64(partNumber)),
			}, nil
		} else {
			if tryNum == maxRetries {
				if aerr, ok := err.(awserr.Error); ok {
					uploadErr = aerr
				} else {
					uploadErr = err
				}
				break
			}
			Log.Warnw("Retrying to upload part", "error", err, "key", resp.Key, "part", partNumber)
			tryNum++
		}
	}
	Log.Errorw("Max retries reached while attempting to upload part", "error", uploadErr, "key", resp.Key, "part", partNumber)
	return nil, uploadErr
}

func abortMultipartUpload(svc *s3.S3, resp *s3.CreateMultipartUploadOutput) error {
	fmt.Println("Aborting multipart upload for UploadId#" + *resp.UploadId)
	abortInput := &s3.AbortMultipartUploadInput{
		Bucket:   resp.Bucket,
		Key:      resp.Key,
		UploadId: resp.UploadId,
	}
	_, err := svc.AbortMultipartUpload(abortInput)
	return err
}
