/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloud

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	//"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/mock/gomock"

	"github.com/outscale/osc-sdk-go/osc"
// 	dm "github.com/outscale-dev/osc-bsu-csi-driver/pkg/cloud/devicemanager"
// 	"github.com/outscale-dev/osc-bsu-csi-driver/pkg/cloud/mocks"
// 	"github.com/outscale-dev/osc-bsu-csi-driver/pkg/util"
	dm "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/devicemanager"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/mocks"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
)

const (
	defaultZone = "test-az"
	expZone     = "us-west-2b"
)

func TestCreateDisk(t *testing.T) {
	testCases := []struct {
		name               string
		volumeName         string
		volState           string
		diskOptions        *DiskOptions
		expDisk            *Disk
		expErr             error
		expCreateVolumeErr error
		expDescVolumeErr   error
	}{
		{
			name:       "success: normal",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(1),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test"},
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: defaultZone,
			},
			expErr: nil,
		},
		{
			name:       "success: normal with provided zone",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
				AvailabilityZone: expZone,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: expZone,
			},
			expErr: nil,
		},
		{
			name:       "success: normal with encrypted volume",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
				AvailabilityZone: expZone,
				Encrypted:        true,
				KmsKeyID:         "arn:aws:kms:us-east-1:012345678910:key/abcd1234-a123-456a-a12b-a123b4cd56ef",
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: expZone,
			},
			expErr: nil,
		},
		{
			name:       "fail: CreateVolume returned CreateVolume error",
			volumeName: "vol-test-name-error",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
				AvailabilityZone: expZone,
			},
			expErr:             fmt.Errorf("could not create volume in EC2: CreateVolume generic error"),
			expCreateVolumeErr: fmt.Errorf("CreateVolume generic error"),
		},
		{
			name:       "fail: CreateVolume returned a DescribeVolumes error",
			volumeName: "vol-test-name-error",
			volState:   "creating",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
				AvailabilityZone: "",
			},
			expErr:             fmt.Errorf("could not create volume in EC2: DescribeVolumes generic error"),
			expCreateVolumeErr: fmt.Errorf("DescribeVolumes generic error"),
		},
		{
			name:       "fail: CreateVolume returned a volume with wrong state",
			volumeName: "vol-test-name-error",
			volState:   "creating",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
				AvailabilityZone: "",
			},
			expErr: fmt.Errorf("failed to get an available volume in EC2: timed out waiting for the condition"),
		},
		{
			name:       "success: normal from snapshot",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
				AvailabilityZone: expZone,
				SnapshotID:       "snapshot-test",
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: expZone,
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			//mockCtrl := gomock.NewController(t)
			//mockOsc := mocks.NewMockOsc(mockCtrl)
			//c := newCloud(mockOsc)

			mockCtrl := gomock.NewController(t)
			mockOscInterface := mocks.NewMockOscInterface(mockCtrl)
			c := newCloud(mockOscInterface)
 			volState := tc.volState
 			if volState == "" {
 				volState = "available"
 			}



			vol := osc.CreateVolumeResponse{
				Volume: osc.Volume{
					VolumeId: tc.diskOptions.Tags[VolumeNameTagKey],
					Size:     int32(util.BytesToGiB(tc.diskOptions.CapacityBytes)),
					State:    volState,
					SubregionName: tc.diskOptions.AvailabilityZone,
				},
 			}

 			readVol := osc.Volume{
					VolumeId: tc.diskOptions.Tags[VolumeNameTagKey],
					Size:     int32(util.BytesToGiB(tc.diskOptions.CapacityBytes)),
					State:    volState,
					SubregionName: tc.diskOptions.AvailabilityZone,
				}

		    readSnapshot := osc.Snapshot{
				SnapshotId: tc.diskOptions.SnapshotID,
				VolumeId:   "snap-test-volume",
				State:      "completed",
			}

			tag := osc.CreateTagsResponse{}
			ctx := context.Background()
			mockOscInterface.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Any()).Return(vol, nil, tc.expCreateVolumeErr)
			mockOscInterface.EXPECT().CreateTags(gomock.Eq(ctx), gomock.Any()).Return(tag, nil, nil).AnyTimes()
			mockOscInterface.EXPECT().ReadVolumes(gomock.Eq(ctx), gomock.Any()).Return(osc.ReadVolumesResponse{Volumes: []osc.Volume{readVol}}, nil, tc.expDescVolumeErr).AnyTimes()
			if len(tc.diskOptions.SnapshotID) > 0 {
				mockOscInterface.EXPECT().ReadSnapshots(gomock.Eq(ctx), gomock.Any()).Return(osc.ReadSnapshotsResponse{Snapshots: []osc.Snapshot{readSnapshot}}, nil, nil).AnyTimes()
			}


			disk, err := c.CreateDisk(ctx, tc.volumeName, tc.diskOptions)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("CreateDisk() failed: expected no error, got: %v", err)
				} else if tc.expErr.Error() != err.Error() {
					t.Fatalf("CreateDisk() failed: expected error %q, got: %q", tc.expErr, err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("CreateDisk() failed: expected error, got nothing")
				} else {
					if tc.expDisk.CapacityGiB != disk.CapacityGiB {
						t.Fatalf("CreateDisk() failed: expected capacity %d, got %d", tc.expDisk.CapacityGiB, disk.CapacityGiB)
					}
					if tc.expDisk.VolumeID != disk.VolumeID {
						t.Fatalf("CreateDisk() failed: expected capacity %q, got %q", tc.expDisk.VolumeID, disk.VolumeID)
					}
					if tc.expDisk.AvailabilityZone != disk.AvailabilityZone {
						t.Fatalf("CreateDisk() failed: expected availabilityZone %q, got %q", tc.expDisk.AvailabilityZone, disk.AvailabilityZone)
					}
				}
			}

			mockCtrl.Finish()
		})
	}
}


func TestDeleteDisk(t *testing.T) {
	testCases := []struct {
		name     string
		volumeID string
		expResp  bool
		expErr   error
	}{
		{
			name:     "success: normal",
			volumeID: "vol-test-1234",
			expResp:  true,
			expErr:   nil,
		},
		{
			name:     "fail: DeleteVolume returned generic error",
			volumeID: "vol-test-1234",
			expResp:  false,
			expErr:   fmt.Errorf("DeleteVolume generic error"),
		},
		{
			name:     "fail: DeleteVolume returned not found error",
			volumeID: "vol-test-1234",
			expResp:  false,
			expErr:   awserr.New("InvalidVolume.NotFound", "", nil),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
// 			mockCtrl := gomock.NewController(t)
// 			mockOsc := mocks.NewMockEC2(mockCtrl)
// 			c := newCloud(mockOsc)

			mockCtrl := gomock.NewController(t)
			mockOscInterface := mocks.NewMockOscInterface(mockCtrl)
			c := newCloud(mockOscInterface)

			ctx := context.Background()
			mockOscInterface.EXPECT().DeleteVolume(gomock.Eq(ctx), gomock.Any()).Return(osc.DeleteVolumeResponse{}, nil, tc.expErr)

			ok, err := c.DeleteDisk(ctx, tc.volumeID)
			if err != nil && tc.expErr == nil {
				t.Fatalf("DeleteDisk() failed: expected no error, got: %v", err)
			}

			if err == nil && tc.expErr != nil {
				t.Fatal("DeleteDisk() failed: expected error, got nothing")
			}

			if tc.expResp != ok {
				t.Fatalf("DeleteDisk() failed: expected return %v, got %v", tc.expResp, ok)
			}

			mockCtrl.Finish()
		})
	}
}


func TestAttachDisk(t *testing.T) {
	testCases := []struct {
		name     string
		volumeID string
		nodeID   string
		expErr   error
	}{
		{
			name:     "success: normal",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   nil,
		},
		{
			name:     "fail: AttachVolume returned generic error",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   fmt.Errorf(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockOscInterface := mocks.NewMockOscInterface(mockCtrl)
			c := newCloud(mockOscInterface)

			vol := osc.Volume{
				VolumeId:    tc.volumeID,
				LinkedVolumes: []osc.LinkedVolume{State: "attached"},
			}

			ctx := context.Background()
			mockOscInterface.EXPECT().ReadVolumes(gomock.Eq(ctx), gomock.Any()).Return(osc.ReadVolumesResponse{Volumes: []osc.Volume{vol}}, nil).AnyTimes()
			mockOscInterface.EXPECT().ReadVms(gomock.Eq(ctx), gomock.Any()).Return(osc.ReadVmsResponse{[]osc.Vm{tc.nodeID}}, nil, nil)
			mockOscInterface.EXPECT().LinkVolume(gomock.Eq(ctx), gomock.Any()).Return(osc.LinkedVolume{}, tc.expErr)

			devicePath, err := c.AttachDisk(ctx, tc.volumeID, tc.nodeID)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("AttachDisk() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("AttachDisk() failed: expected error, got nothing")
				}
				if !strings.HasPrefix(devicePath, "/dev/") {
					t.Fatal("AttachDisk() failed: expected valid device path, got empty string")
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestDetachDisk(t *testing.T) {
	testCases := []struct {
		name     string
		volumeID string
		nodeID   string
		expErr   error
	}{
		{
			name:     "success: normal",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   nil,
		},
		{
			name:     "fail: DetachVolume returned generic error",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   fmt.Errorf("DetachVolume generic error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockOsc := mocks.NewMockEC2(mockCtrl)
			c := newCloud(mockOsc)

			vol := &osc.Volume{
				VolumeId:    aws.String(tc.volumeID),
				LinkedVolume: nil,
			}

			ctx := context.Background()
			mockOsc.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.ReadVolumesResponse{Volumes: []osc.Volume{vol}}, nil).AnyTimes()
			mockOsc.EXPECT().DescribeInstancesWithContext(gomock.Eq(ctx), gomock.Any()).Return(newDescribeInstancesOutput(tc.nodeID), nil)
			mockOsc.EXPECT().DetachVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.LinkedVolume{}, tc.expErr)

			err := c.DetachDisk(ctx, tc.volumeID, tc.nodeID)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("DetachDisk() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("DetachDisk() failed: expected error, got nothing")
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestGetDiskByName(t *testing.T) {
	testCases := []struct {
		name             string
		volumeName       string
		volumeCapacity   int64
		availabilityZone string
		expErr           error
	}{
		{
			name:             "success: normal",
			volumeName:       "vol-test-1234",
			volumeCapacity:   util.GiBToBytes(1),
			availabilityZone: expZone,
			expErr:           nil,
		},
		{
			name:           "fail: DescribeVolumes returned generic error",
			volumeName:     "vol-test-1234",
			volumeCapacity: util.GiBToBytes(1),
			expErr:         fmt.Errorf("DescribeVolumes generic error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockOsc := mocks.NewMockEC2(mockCtrl)
			c := newCloud(mockOsc)

			vol := &osc.Volume{
				VolumeId:         tc.volumeName,
				Size:             util.BytesToGiB(tc.volumeCapacity),
				SubregionName:    tc.availabilityZone,
			}

			ctx := context.Background()
			mockOsc.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.ReadVolumesResponse{Volumes: []osc.Volume{vol}}, tc.expErr)

			disk, err := c.GetDiskByName(ctx, tc.volumeName, tc.volumeCapacity)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetDiskByName() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetDiskByName() failed: expected error, got nothing")
				}
				if disk.CapacityGiB != util.BytesToGiB(tc.volumeCapacity) {
					t.Fatalf("GetDiskByName() failed: expected capacity %d, got %d", util.BytesToGiB(tc.volumeCapacity), disk.CapacityGiB)
				}
				if tc.availabilityZone != disk.AvailabilityZone {
					t.Fatalf("GetDiskByName() failed: expected availabilityZone %q, got %q", tc.availabilityZone, disk.AvailabilityZone)
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestGetDiskByID(t *testing.T) {
	testCases := []struct {
		name             string
		volumeID         string
		availabilityZone string
		expErr           error
	}{
		{
			name:             "success: normal",
			volumeID:         "vol-test-1234",
			availabilityZone: expZone,
			expErr:           nil,
		},
		{
			name:     "fail: DescribeVolumes returned generic error",
			volumeID: "vol-test-1234",
			expErr:   fmt.Errorf("DescribeVolumes generic error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockOsc := mocks.NewMockEC2(mockCtrl)
			c := newCloud(mockOsc)

			ctx := context.Background()
			mockOsc.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(
				&osc.ReadVolumesResponse{
					Volumes: []osc.Volume{
						{
							VolumeId:         tc.volumeID,
							SubregionName:    tc.availabilityZone,
						},
					},
				},
				tc.expErr,
			)

			disk, err := c.GetDiskByID(ctx, tc.volumeID)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetDisk() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetDisk() failed: expected error, got nothing")
				}
				if disk.VolumeID != tc.volumeID {
					t.Fatalf("GetDisk() failed: expected ID %q, got %q", tc.volumeID, disk.VolumeID)
				}
				if tc.availabilityZone != disk.AvailabilityZone {
					t.Fatalf("GetDiskByName() failed: expected availabilityZone %q, got %q", tc.availabilityZone, disk.AvailabilityZone)
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestCreateSnapshot(t *testing.T) {
	testCases := []struct {
		name            string
		snapshotName    string
		snapshotOptions *SnapshotOptions
		expSnapshot     *Snapshot
		expErr          error
	}{
		{
			name:         "success: normal",
			snapshotName: "snap-test-name",
			snapshotOptions: &SnapshotOptions{
				Tags: map[string]string{
					SnapshotNameTagKey: "snap-test-name",
				},
			},
			expSnapshot: &Snapshot{
				SourceVolumeID: "snap-test-volume",
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockOsc := mocks.NewMockEC2(mockCtrl)
			c := newCloud(mockOsc)

			oscsnapshot := &osc.Snapshot{
				SnapshotId: tc.snapshotOptions.Tags[SnapshotNameTagKey],
				VolumeId:   "snap-test-volume",
				State:      "completed",
			}

			tag := &osc.CreateTagsResponse{}
			ctx := context.Background()
			mockOsc.EXPECT().CreateSnapshotWithContext(gomock.Eq(ctx), gomock.Any()).Return(oscsnapshot, tc.expErr)
			mockOsc.EXPECT().CreateTagsWithContext(gomock.Eq(ctx), gomock.Any()).Return(tag, nil).AnyTimes()
			mockOsc.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.ReadSnapshotsResponse{Snapshots: []osc.Snapshot{oscsnapshot}}, nil).AnyTimes()

			snapshot, err := c.CreateSnapshot(ctx, tc.expSnapshot.SourceVolumeID, tc.snapshotOptions)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("CreateSnapshot() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("CreateSnapshot() failed: expected error, got nothing")
				} else {
					if snapshot.SourceVolumeID != tc.expSnapshot.SourceVolumeID {
						t.Fatalf("CreateSnapshot() failed: expected source volume ID %s, got %v", tc.expSnapshot.SourceVolumeID, snapshot.SourceVolumeID)
					}
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestDeleteSnapshot(t *testing.T) {
	testCases := []struct {
		name         string
		snapshotName string
		expErr       error
	}{
		{
			name:         "success: normal",
			snapshotName: "snap-test-name",
			expErr:       nil,
		},
		{
			name:         "fail: delete snapshot return generic error",
			snapshotName: "snap-test-name",
			expErr:       fmt.Errorf("DeleteSnapshot generic error"),
		},
		{
			name:         "fail: delete snapshot return not found error",
			snapshotName: "snap-test-name",
			expErr:       awserr.New("InvalidSnapshot.NotFound", "", nil),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockOsc := mocks.NewMockOsc(mockCtrl)
			c := newCloud(mockOsc)

			ctx := context.Background()
			mockOsc.EXPECT().DeleteSnapshotWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.DeleteSnapshotResponse{}, tc.expErr)

			_, err := c.DeleteSnapshot(ctx, tc.snapshotName)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("DeleteSnapshot() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("DeleteSnapshot() failed: expected error, got nothing")
				}
			}

			mockCtrl.Finish()
		})
	}
}

// Volume Modification not implemented
// func TestResizeDisk(t *testing.T) {
// 	testCases := []struct {
// 		name                string
// 		volumeID            string
// 		existingVolume      *osc.Volume
// 		existingVolumeError awserr.Error
// 		modifiedVolume      *osc.ModifyVolumeOutput
// 		modifiedVolumeError awserr.Error
// 		descModVolume       *osc.DescribeVolumesModificationsOutput
// 		reqSizeGiB          int64
// 		expErr              error
// 	}{
// 		{
// 			name:     "success: normal",
// 			volumeID: "vol-test",
// 			existingVolume: &osc.Volume{
// 				VolumeId:         "vol-test",
// 				Size:             1,
// 				SubregionName: defaultZone,
// 			},
// 			modifiedVolume: &osc.ModifyVolumeOutput{
// 				VolumeModification: &ec2.VolumeModification{
// 					VolumeId:          aws.String("vol-test"),
// 					TargetSize:        aws.Int64(2),
// 					ModificationState: aws.String(ec2.VolumeModificationStateOptimizing),
// 				},
// 			},
// 			reqSizeGiB: 2,
// 			expErr:     nil,
// 		},
// 		{
// 			name:     "success: normal modifying state",
// 			volumeID: "vol-test",
// 			existingVolume: &ec2.Volume{
// 				VolumeId:         aws.String("vol-test"),
// 				Size:             aws.Int64(1),
// 				AvailabilityZone: aws.String(defaultZone),
// 			},
// 			modifiedVolume: &ec2.ModifyVolumeOutput{
// 				VolumeModification: &ec2.VolumeModification{
// 					VolumeId:          aws.String("vol-test"),
// 					TargetSize:        aws.Int64(2),
// 					ModificationState: aws.String(ec2.VolumeModificationStateModifying),
// 				},
// 			},
// 			descModVolume: &ec2.DescribeVolumesModificationsOutput{
// 				VolumesModifications: []*ec2.VolumeModification{
// 					{
// 						VolumeId:          aws.String("vol-test"),
// 						TargetSize:        aws.Int64(2),
// 						ModificationState: aws.String(ec2.VolumeModificationStateCompleted),
// 					},
// 				},
// 			},
// 			reqSizeGiB: 2,
// 			expErr:     nil,
// 		},
// 		{
// 			name:                "fail: volume doesn't exist",
// 			volumeID:            "vol-test",
// 			existingVolumeError: awserr.New("InvalidVolume.NotFound", "", nil),
// 			reqSizeGiB:          2,
// 			expErr:              fmt.Errorf("ResizeDisk generic error"),
// 		},
// 		{
// 			name:     "success: there is a resizing in progress",
// 			volumeID: "vol-test",
// 			existingVolume: &ec2.Volume{
// 				VolumeId:         aws.String("vol-test"),
// 				Size:             aws.Int64(1),
// 				AvailabilityZone: aws.String(defaultZone),
// 			},
// 			modifiedVolumeError: awserr.New("IncorrectModificationState", "", nil),
// 			descModVolume: &ec2.DescribeVolumesModificationsOutput{
// 				VolumesModifications: []*ec2.VolumeModification{
// 					{
// 						VolumeId:          aws.String("vol-test"),
// 						TargetSize:        aws.Int64(2),
// 						ModificationState: aws.String(ec2.VolumeModificationStateCompleted),
// 					},
// 				},
// 			},
// 			reqSizeGiB: 2,
// 			expErr:     nil,
// 		},
// 	}
//
// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			mockCtrl := gomock.NewController(t)
// 			mockOsc := mocks.NewMockEC2(mockCtrl)
// 			c := newCloud(mockOsc)
//
// 			ctx := context.Background()
// 			if tc.existingVolume != nil || tc.existingVolumeError != nil {
// 				mockOsc.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(
// 					&ec2.DescribeVolumesOutput{
// 						Volumes: []*ec2.Volume{tc.existingVolume},
// 					}, tc.existingVolumeError).AnyTimes()
// 			}
// 			if tc.modifiedVolume != nil || tc.modifiedVolumeError != nil {
// 				mockOsc.EXPECT().ModifyVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(tc.modifiedVolume, tc.modifiedVolumeError).AnyTimes()
// 			}
// 			if tc.descModVolume != nil {
// 				mockOsc.EXPECT().DescribeVolumesModificationsWithContext(gomock.Eq(ctx), gomock.Any()).Return(tc.descModVolume, nil).AnyTimes()
// 			}
//
// 			newSize, err := c.ResizeDisk(ctx, tc.volumeID, util.GiBToBytes(tc.reqSizeGiB))
// 			if err != nil {
// 				if tc.expErr == nil {
// 					t.Fatalf("ResizeDisk() failed: expected no error, got: %v", err)
// 				}
// 			} else {
// 				if tc.expErr != nil {
// 					t.Fatal("ResizeDisk() failed: expected error, got nothing")
// 				} else {
// 					if tc.reqSizeGiB != newSize {
// 						t.Fatalf("ResizeDisk() failed: expected capacity %d, got %d", tc.reqSizeGiB, newSize)
// 					}
// 				}
// 			}
//
// 			mockCtrl.Finish()
// 		})
// 	}
// }

func TestGetSnapshotByName(t *testing.T) {
	testCases := []struct {
		name            string
		snapshotName    string
		snapshotOptions *SnapshotOptions
		expSnapshot     *Snapshot
		expErr          error
	}{
		{
			name:         "success: normal",
			snapshotName: "snap-test-name",
			snapshotOptions: &SnapshotOptions{
				Tags: map[string]string{
					SnapshotNameTagKey: "snap-test-name",
				},
			},
			expSnapshot: &Snapshot{
				SourceVolumeID: "snap-test-volume",
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockOsc := mocks.NewMockEC2(mockCtrl)
			c := newCloud(mockOsc)

			oscsnapshot := &osc.Snapshot{
				SnapshotId: tc.snapshotOptions.Tags[SnapshotNameTagKey],
				VolumeId:   "snap-test-volume",
				State:      "completed",
			}

			ctx := context.Background()
			mockOsc.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.ReadSnapshotsResponse{Snapshots: []osc.Snapshot{oscsnapshot}}, nil)

			_, err := c.GetSnapshotByName(ctx, tc.snapshotOptions.Tags[SnapshotNameTagKey])
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetSnapshotByName() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetSnapshotByName() failed: expected error, got nothing")
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestGetSnapshotByID(t *testing.T) {
	testCases := []struct {
		name            string
		snapshotName    string
		snapshotOptions *SnapshotOptions
		expSnapshot     *Snapshot
		expErr          error
	}{
		{
			name:         "success: normal",
			snapshotName: "snap-test-name",
			snapshotOptions: &SnapshotOptions{
				Tags: map[string]string{
					SnapshotNameTagKey: "snap-test-name",
				},
			},
			expSnapshot: &Snapshot{
				SourceVolumeID: "snap-test-volume",
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockOsc := mocks.NewMockEC2(mockCtrl)
			c := newCloud(mockOsc)

			oscsnapshot := &osc.Snapshot{
				SnapshotId: tc.snapshotOptions.Tags[SnapshotNameTagKey],
				VolumeId:   "snap-test-volume",
				State:      "completed",
			}

			ctx := context.Background()
			mockOsc.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.ReadSnapshotsResponse{Snapshots: []osc.Snapshot{oscsnapshot}}, nil)

			_, err := c.GetSnapshotByID(ctx, tc.snapshotOptions.Tags[SnapshotNameTagKey])
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetSnapshotByName() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetSnapshotByName() failed: expected error, got nothing")
				}
			}

			mockCtrl.Finish()
		})
	}
}
func TestListSnapshots(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: normal",
			testFunc: func(t *testing.T) {
				expSnapshots := []*Snapshot{
					{
						SourceVolumeID: "snap-test-volume1",
						SnapshotID:     "snap-test-name1",
					},
					{
						SourceVolumeID: "snap-test-volume2",
						SnapshotID:     "snap-test-name2",
					},
				}
				oscsnapshot := []*osc.Snapshot{
					{
						SnapshotId: expSnapshots[0].SnapshotID,
						VolumeId:   "snap-test-volume1",
						State:      "completed",
					},
					{
						SnapshotId: expSnapshots[1].SnapshotID,
						VolumeId:   "snap-test-volume2",
						State:      "completed",
					},
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockOsc := mocks.NewMockEC2(mockCtl)
				c := newCloud(mockOsc)

				ctx := context.Background()

				mockOsc.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.ReadSnapshotsResponse{Snapshots: oscsnapshot}, nil)

				_, err := c.ListSnapshots(ctx, "", 0, "")
				if err != nil {
					t.Fatalf("ListSnapshots() failed: expected no error, got: %v", err)
				}
			},
		},
		{
			name: "success: with volume ID",
			testFunc: func(t *testing.T) {
				sourceVolumeID := "snap-test-volume"
				expSnapshots := []*Snapshot{
					{
						SourceVolumeID: sourceVolumeID,
						SnapshotID:     "snap-test-name1",
					},
					{
						SourceVolumeID: sourceVolumeID,
						SnapshotID:     "snap-test-name2",
					},
				}
				oscsnapshot := []*osc.Snapshot{
					{
						SnapshotId: expSnapshots[0].SnapshotID,
						VolumeId:   sourceVolumeID,
						State:      "completed",
					},
					{
						SnapshotId: expSnapshots[1].SnapshotID,
						VolumeId:   sourceVolumeID,
						State:      "completed",
					},
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockOsc := mocks.NewMockEC2(mockCtl)
				c := newCloud(mockOsc)

				ctx := context.Background()

				mockOsc.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.ReadSnapshotsResponse{Snapshots: oscsnapshot}, nil)

				resp, err := c.ListSnapshots(ctx, sourceVolumeID, 0, "")
				if err != nil {
					t.Fatalf("ListSnapshots() failed: expected no error, got: %v", err)
				}

				if len(resp.Snapshots) != len(expSnapshots) {
					t.Fatalf("Expected %d snapshots, got %d", len(expSnapshots), len(resp.Snapshots))
				}

				for _, snap := range resp.Snapshots {
					if snap.SourceVolumeID != sourceVolumeID {
						t.Fatalf("Unexpected source volume.  Expected %s, got %s", sourceVolumeID, snap.SourceVolumeID)
					}
				}
			},
		},
		{
			name: "success: max results, next token",
			testFunc: func(t *testing.T) {
				maxResults := 5
				nextTokenValue := "nextTokenValue"
				var expSnapshots []*Snapshot
				for i := 0; i < maxResults*2; i++ {
					expSnapshots = append(expSnapshots, &Snapshot{
						SourceVolumeID: "snap-test-volume1",
						SnapshotID:     fmt.Sprintf("snap-test-name%d", i),
					})
				}

				var oscsnapshots []osc.Snapshot
				for i := 0; i < maxResults*2; i++ {
					oscsnapshot = append(oscsnapshots, &osc.Snapshot{
						SnapshotId: expSnapshots[i].SnapshotID,
						VolumeId:   fmt.Sprintf("snap-test-volume%d", i),
						State:      "completed",
					})
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockOsc := mocks.NewMockEC2(mockCtl)
				c := newCloud(mockOsc)

				ctx := context.Background()

				firstCall := mockOsc.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.ReadSnapshotsResponse{
					Snapshots: oscsnapshots[:maxResults],
					NextToken: aws.String(nextTokenValue),
				}, nil)
				secondCall := mockOsc.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.ReadSnapshotsResponse{
					Snapshots: oscsnapshots[maxResults:],
				}, nil)
				gomock.InOrder(
					firstCall,
					secondCall,
				)

				firstSnapshotsResponse, err := c.ListSnapshots(ctx, "", 5, "")
				if err != nil {
					t.Fatalf("ListSnapshots() failed: expected no error, got: %v", err)
				}

				if len(firstSnapshotsResponse.Snapshots) != maxResults {
					t.Fatalf("Expected %d snapshots, got %d", maxResults, len(firstSnapshotsResponse.Snapshots))
				}

				if firstSnapshotsResponse.NextToken != nextTokenValue {
					t.Fatalf("Expected next token value '%s' got '%s'", nextTokenValue, firstSnapshotsResponse.NextToken)
				}

				secondSnapshotsResponse, err := c.ListSnapshots(ctx, "", 0, firstSnapshotsResponse.NextToken)
				if err != nil {
					t.Fatalf("CreateSnapshot() failed: expected no error, got: %v", err)
				}

				if len(secondSnapshotsResponse.Snapshots) != maxResults {
					t.Fatalf("Expected %d snapshots, got %d", maxResults, len(secondSnapshotsResponse.Snapshots))
				}

				if secondSnapshotsResponse.NextToken != "" {
					t.Fatalf("Expected next token value to be empty got %s", secondSnapshotsResponse.NextToken)
				}
			},
		},
		{
			name: "fail: AWS DescribeSnapshotsWithContext error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockOsc := mocks.NewMockEC2(mockCtl)
				c := newCloud(mockOsc)

				ctx := context.Background()

				mockOsc.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("test error"))

				if _, err := c.ListSnapshots(ctx, "", 0, ""); err == nil {
					t.Fatalf("ListSnapshots() failed: expected an error, got none")
				}
			},
		},
		{
			name: "fail: no snapshots ErrNotFound",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockOsc := mocks.NewMockEC2(mockCtl)
				c := newCloud(mockOsc)

				ctx := context.Background()

				mockOsc.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&osc.ReadSnapshotsResponse{}, nil)

				if _, err := c.ListSnapshots(ctx, "", 0, ""); err != nil {
					if err != ErrNotFound {
						t.Fatalf("Expected error %v, got %v", ErrNotFound, err)
					}
				} else {
					t.Fatalf("Expected error, got none")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}


//func newCloud(mock *mocks.MockVolumeAPI) Cloud {
// 	client := &OscClient{}
// 	client.config = osc.NewConfiguration()
// 	client.auth = context.WithValue(context.Background(), osc.ContextAWSv4, osc.AWSv4{
// 		AccessKey: ("OSC_ACCESS_KEY"),
// 		SecretKey: ("OSC_SECRET_KEY"),
// 	})
// 	client.api = osc.NewAPIClient(client.config)
//
// 	client.api.VolumeApi = mock
//
// 	return &cloud{
// 		region: "test-region",
// 		dm:     dm.NewDeviceManager(),
// 		client: OscClient,
// 		metadata: &Metadata{
// 			InstanceID:       "test-instance",
// 			Region:           "test-region",
// 			AvailabilityZone: defaultZone,
// 		},
// 	}
// }
func newCloud(mockOscInterface OscInterface) *cloud {
	client := &OscClient{}
	client.config = osc.NewConfiguration()
	client.api = osc.NewAPIClient(client.config)
	client.auth = context.WithValue(context.Background(), osc.ContextAWSv4, osc.AWSv4{
		AccessKey: "OSC_ACCESS_KEY",
		SecretKey: "OSC_SECRET_KEY",
	})


	return &cloud{
		region:   "test-region",
		dm:       dm.NewDeviceManager(),
		clientIf: mockOscInterface,
		metadata: &Metadata{
			InstanceID:       "test-instance",
			Region:           "test-region",
			AvailabilityZone: defaultZone,
		},
		//Remove when compile pass
		client:   client,

	}
}


// func newDescribeInstancesOutput(nodeID string) *ec2.DescribeInstancesOutput {
// 	return &ec2.DescribeInstancesOutput{
// 		Reservations: []*ec2.Reservation{{
// 			Instances: []*ec2.Instance{
// 				{InstanceId: aws.String(nodeID)},
// 			},
// 		}},
// 	}
// }

func newDescribeInstancesOutput(nodeID string) osc.ReadVmsResponse {
	return osc.ReadVmsResponse{
		Vms: []osc.Vms{
				{VmId: nodeID},
		},
	}
}
