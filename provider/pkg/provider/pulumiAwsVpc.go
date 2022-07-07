// Copyright 2016-2021, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/ec2"
	p "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Set of arguments for created individual subnets in an availability zone.
type SubnetAvailabilityZoneConfig struct {
	AvailabilityZone   p.StringInput `pulumi:"availabilityZone"`
	PublicSubnetCidr   p.StringInput `pulumi:"publicSubnetCidr"`
	PrivateSubnetCidr  p.StringInput `pulumi:"privateSubnetCidr"`
	IsolatedSubnetCidr p.StringInput `pulumi:"isolatedSubnetCidr"`
	CreateNatGateway   *bool         `pulumi:"createNatGateway"`
}

// The set of arguments for creating a Pulumi AWS VPC component resource.
type PulumiAwsVpcArgs struct {
	CidrBlock                    p.StringInput                  `pulumi:"cidrBlock"`
	CreatePublicSubnets          *bool                          `pulumi:"createPublicSubnets"`
	CreatePrivateSubnets         *bool                          `pulumi:"createPrivateSubnets"`
	CreateIsolatedSubnets        *bool                          `pulumi:"createIsolatedSubnets"`
	EnableDnsHostnames           *bool                          `pulumi:"enableDnsHostnames"`
	EnableDnsSupport             *bool                          `pulumi:"enableDnsSupport"`
	SubnetAvailabilityZoneConfig []SubnetAvailabilityZoneConfig `pulumi:"subnetAvailabilityZoneConfig"`
	InstanceTenancy              p.StringInput                  `pulumi:"instanceTenancy"`
}

// The StaticPage component resource.
type PulumiAwsVpc struct {
	p.ResourceState

	VpcId             p.StringOutput      `pulumi:"vpcId"`
	PrivateSubnetIDs  p.StringArrayOutput `pulumi:"privateSubnetIds"`
	PublicSubnetIDs   p.StringArrayOutput `pulumi:"publicSubnetIds"`
	IsolatedSubnetIds p.StringArrayOutput `pulumi:"isolatedSubnetIds"`
	NatGatewayIds     p.StringArrayOutput `pulumi:"natGatewayIds"`
}

// NewStaticPage creates a new StaticPage component resource.
func NewPulubmiAwsVpc(ctx *p.Context,
	name string, args *PulumiAwsVpcArgs, opts ...p.ResourceOption) (*PulumiAwsVpc, error) {
	if args == nil {
		args = &PulumiAwsVpcArgs{}
	}

	if args.CidrBlock == nil {
		return nil, fmt.Errorf("cidr block must be a valid non-nil value")
	}

	component := &PulumiAwsVpc{}
	err := ctx.RegisterComponentResource("pulumi-aws-vpc:index:AwsVpc", name, component, opts...)
	if err != nil {
		return nil, err
	}

	tenancy := p.String("default").ToStringOutput()
	if args.InstanceTenancy != nil {
		tenancy = args.InstanceTenancy.ToStringOutput()
	}

	enabledDnsHostnames := true
	if args.EnableDnsHostnames != nil {
		enabledDnsHostnames = *args.EnableDnsHostnames
	}

	enableDnsSupport := true
	if args.EnableDnsSupport != nil {
		enableDnsSupport = *args.EnableDnsSupport
	}

	vpc, err := ec2.NewVpc(ctx, fmt.Sprintf("%s-vpc", name), &ec2.VpcArgs{
		InstanceTenancy:    tenancy,
		CidrBlock:          args.CidrBlock,
		EnableDnsSupport:   p.Bool(enableDnsSupport),
		EnableDnsHostnames: p.Bool(enabledDnsHostnames),
	}, p.Parent(component))

	if err != nil {
		return nil, err
	}

	igw, err := ec2.NewInternetGateway(ctx, fmt.Sprintf("%s-igw", name), &ec2.InternetGatewayArgs{
		VpcId: vpc.ID(),
	}, p.Parent(component))

	if err != nil {
		return nil, err
	}

	createPublicSubnets := true
	createPrivateSubnets := true
	createIsolatedSubnets := true

	if args.CreatePrivateSubnets != nil {
		createPublicSubnets = *args.CreatePublicSubnets
	}

	if args.CreatePrivateSubnets != nil {
		createPrivateSubnets = *args.CreatePrivateSubnets
	}

	if args.CreateIsolatedSubnets != nil {
		createIsolatedSubnets = *args.CreateIsolatedSubnets
	}

	var publicSubnets []p.StringOutput
	var privateSubnets []p.StringOutput
	var isolatedSubnets []p.StringOutput
	var gatewayIps []p.StringOutput

	for i, az := range args.SubnetAvailabilityZoneConfig {

		if az.AvailabilityZone == nil {
			ctx.Log.Debug("availability zone config is nil; no subnets created", nil)
		}

		var nat *ec2.NatGateway
		if createPublicSubnets && az.PublicSubnetCidr != nil {
			sub, err := newSubnet(ctx, fmt.Sprintf("%s-public-%d", name, i), vpc.ID().ToStringOutput(), igw.ID().ToStringOutput(), az, component)
			if err != nil {
				return nil, err
			}

			// create natgateway
			if az.CreateNatGateway != nil && *az.CreateNatGateway {
				ip, err := ec2.NewEip(ctx, fmt.Sprintf("%s-eip-%d", name, i), &ec2.EipArgs{
					Vpc: p.Bool(true),
				}, p.Parent(component))

				if err != nil {
					return nil, err
				}

				nat, err := ec2.NewNatGateway(ctx, fmt.Sprintf("%s-natgateway-%d", name, i), &ec2.NatGatewayArgs{
					AllocationId: ip.AllocationId,
					SubnetId:     sub.ID(),
				}, p.Parent(component))

				if err != nil {
					return nil, err
				}

				gatewayIps = append(gatewayIps, nat.ID().ToStringOutput())
			}

			publicSubnets = append(publicSubnets, sub.ID().ToStringOutput())
		}

		if createPrivateSubnets && az.PrivateSubnetCidr != nil {
			sub, err := newSubnet(ctx, fmt.Sprintf("%s-private-%d", name, i), vpc.ID().ToStringOutput(), nat, az, component)
			if err != nil {
				return nil, err
			}

			privateSubnets = append(privateSubnets, sub.ID().ToStringOutput())
		}

		if createIsolatedSubnets && az.IsolatedSubnetCidr != nil {
			sub, err := newSubnet(ctx, fmt.Sprintf("%s-isolated-%d", name, i), vpc.ID().ToStringOutput(), nil, az, component)
			if err != nil {
				return nil, err
			}

			isolatedSubnets = append(isolatedSubnets, sub.ID().ToStringOutput())
		}
	}

	component.VpcId = vpc.ID().ToStringOutput()
	component.PublicSubnetIDs = p.ToStringArrayOutput(publicSubnets)
	component.PrivateSubnetIDs = p.ToStringArrayOutput(privateSubnets)
	component.IsolatedSubnetIds = p.ToStringArrayOutput(isolatedSubnets)

	// // Create a bucket and expose a website index document.
	// bucket, err := s3.NewBucket(ctx, name, &s3.BucketArgs{
	// 	Website: s3.BucketWebsiteArgs{
	// 		IndexDocument: pulumi.String("index.html"),
	// 	},
	// }, pulumi.Parent(component))
	// if err != nil {
	// 	return nil, err
	// }

	// // Create a bucket object for the index document.
	// if _, err := s3.NewBucketObject(ctx, name, &s3.BucketObjectArgs{
	// 	Bucket:      bucket.ID(),
	// 	Key:         pulumi.String("index.html"),
	// 	Content:     args.IndexContent,
	// 	ContentType: pulumi.String("text/html"),
	// }, pulumi.Parent(bucket)); err != nil {
	// 	return nil, err
	// }

	// // Set the access policy for the bucket so all objects are readable.
	// if _, err := s3.NewBucketPolicy(ctx, "bucketPolicy", &s3.BucketPolicyArgs{
	// 	Bucket: bucket.ID(),
	// 	Policy: pulumi.Any(map[string]interface{}{
	// 		"Version": "2012-10-17",
	// 		"Statement": []map[string]interface{}{
	// 			{
	// 				"Effect":    "Allow",
	// 				"Principal": "*",
	// 				"Action": []interface{}{
	// 					"s3:GetObject",
	// 				},
	// 				"Resource": []interface{}{
	// 					pulumi.Sprintf("arn:aws:s3:::%s/*", bucket.ID()), // policy refers to bucket name explicitly
	// 				},
	// 			},
	// 		},
	// 	}),
	// }, pulumi.Parent(bucket)); err != nil {
	// 	return nil, err
	// }

	// component.Bucket = bucket
	// component.WebsiteUrl = bucket.WebsiteEndpoint

	// if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
	// 	"bucket":     bucket,
	// 	"websiteUrl": bucket.WebsiteEndpoint,
	// }); err != nil {
	// 	return nil, err
	// }

	return component, nil
}

func newSubnet(ctx *p.Context, name string, vpcId p.StringOutput, gatewayId *p.StringOutput, az SubnetAvailabilityZoneConfig, component *PulumiAwsVpc) (*ec2.Subnet, error) {
	sub, err := ec2.NewSubnet(ctx, fmt.Sprintf("%s-subnet", name), &ec2.SubnetArgs{
		VpcId:            vpcId,
		AvailabilityZone: az.AvailabilityZone,
		CidrBlock:        az.PublicSubnetCidr,
	}, p.Parent(component))

	if err != nil {
		return nil, err
	}

	if gatewayId != nil {

	}

	rt, err := ec2.NewRouteTable(ctx, fmt.Sprint("%s-rt", name), &ec2.RouteTableArgs{
		VpcId: vpcId,
	}, p.Parent(component))

	if err != nil {
		return nil, err
	}

	_, err = ec2.NewRoute(ctx, fmt.Sprintf("%s-route", name), &ec2.RouteArgs{
		RouteTableId:         rt.ID(),
		DestinationCidrBlock: p.String("0.0.0.0/0"),
		GatewayId:            gatewayId,
	}, p.Parent(component))

	if err != nil {
		return nil, err
	}

	_, err = ec2.NewRouteTableAssociation(ctx, fmt.Sprintf("%s-rt-assoc", name), &ec2.RouteTableAssociationArgs{
		RouteTableId: rt.ID(),
		SubnetId:     sub.ID(),
	}, p.Parent(component))

	if err != nil {
		return nil, err
	}

	return sub, nil
}
