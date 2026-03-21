package handlers

// resource_collector.go — mirrors the standalone multi-cloud report script exactly.
// Fetches LIVE resource counts from AWS/Azure/GCP using their native SDKs.
// These counts are used for email report tiles and the /resources endpoint.

import (
	"context"
	"log"
	"strings"
	"sync"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armappservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appservice/armappservice/v4"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	armsql "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	armstorage "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"

	"fmt"

	compute_gcp "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	container "cloud.google.com/go/container/apiv1"
	containerpb "cloud.google.com/go/container/apiv1/containerpb"
	gcpstorage "cloud.google.com/go/storage"
	"google.golang.org/api/cloudfunctions/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	sqladmin "google.golang.org/api/sqladmin/v1"
)

// ── Result structs ────────────────────────────────────────────────────────────

// AWSResourceCounts holds live counts fetched directly from AWS APIs.
type AWSResourceCounts struct {
	EC2Instances  int
	RunningEC2    int
	RDSInstances  int
	LoadBalancers int
	EKSClusters   int
	S3Buckets     int
	LambdaCount   int
	ElasticIPs    int
	Snapshots     int
	EBSStorageGB  int64
	ActiveRegions int
	TotalRegions  int
}

// AzureResourceCounts holds live counts fetched directly from Azure APIs.
type AzureResourceCounts struct {
	VMs             int
	ManagedDisks    int
	Snapshots       int
	PublicIPs       int
	AKSClusters     int
	Databases       int
	AppServices     int
	StorageAccounts int
}

// GCPResourceCounts holds live counts fetched directly from GCP APIs.
type GCPResourceCounts struct {
	VMInstances    int
	Disks          int
	Snapshots      int
	GKEClusters    int
	CloudFunctions int
	SQLInstances   int
	GCSBuckets     int
}

// ── AWS ───────────────────────────────────────────────────────────────────────

func CollectAWSResources(ctx context.Context, creds map[string]string) AWSResourceCounts {
	r := AWSResourceCounts{}

	baseCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds["access_key_id"], creds["secret_access_key"], creds["session_token"],
		)),
	)
	if err != nil {
		log.Printf("[resource_collector/aws] config error: %v", err)
		return r
	}

	// Discover all regions
	var regions []string
	if out, err := ec2.NewFromConfig(baseCfg).DescribeRegions(ctx, &ec2.DescribeRegionsInput{}); err == nil {
		for _, reg := range out.Regions {
			regions = append(regions, awssdk.ToString(reg.RegionName))
		}
	} else {
		regions = []string{"us-east-1"}
	}
	r.TotalRegions = len(regions)

	// S3 is global
	if out, err := s3.NewFromConfig(baseCfg).ListBuckets(ctx, &s3.ListBucketsInput{}); err == nil {
		r.S3Buckets = len(out.Buckets)
	}

	type regionResult struct {
		ec2Count  int
		running   int
		ebsGB     int64
		snapshots int
		eips      int
		rds       int
		lbs       int
		eks       int
		lambdas   int
		hasEC2    bool
	}

	ch := make(chan regionResult, len(regions))
	var wg sync.WaitGroup

	for _, region := range regions {
		wg.Add(1)
		go func(reg string) {
			defer wg.Done()
			res := regionResult{}

			rc, err := awsconfig.LoadDefaultConfig(ctx,
				awsconfig.WithRegion(reg),
				awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
					creds["access_key_id"], creds["secret_access_key"], creds["session_token"],
				)),
			)
			if err != nil {
				return
			}

			regEC2 := ec2.NewFromConfig(rc)

			if out, err := regEC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{}); err == nil {
				for _, resv := range out.Reservations {
					for _, inst := range resv.Instances {
						res.ec2Count++
						if inst.State.Name == ec2types.InstanceStateNameRunning {
							res.running++
						}
						res.hasEC2 = true
					}
				}
			}
			if out, err := regEC2.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{}); err == nil {
				for _, v := range out.Volumes {
					if v.Size != nil {
						res.ebsGB += int64(*v.Size)
					}
				}
			}
			if out, err := regEC2.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{OwnerIds: []string{"self"}}); err == nil {
				res.snapshots = len(out.Snapshots)
			}
			if out, err := regEC2.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{}); err == nil {
				res.eips = len(out.Addresses)
			}
			if out, err := rds.NewFromConfig(rc).DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{}); err == nil {
				res.rds = len(out.DBInstances)
			}
			if out, err := elasticloadbalancingv2.NewFromConfig(rc).DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{}); err == nil {
				res.lbs = len(out.LoadBalancers)
			}
			eksC := eks.NewFromConfig(rc)
			if out, err := eksC.ListClusters(ctx, &eks.ListClustersInput{}); err == nil {
				res.eks = len(out.Clusters)
			}
			if out, err := lambda.NewFromConfig(rc).ListFunctions(ctx, &lambda.ListFunctionsInput{}); err == nil {
				res.lambdas = len(out.Functions)
			}

			ch <- res
		}(region)
	}

	go func() { wg.Wait(); close(ch) }()

	activeRegions := 0
	for res := range ch {
		r.EC2Instances += res.ec2Count
		r.RunningEC2 += res.running
		r.EBSStorageGB += res.ebsGB
		r.Snapshots += res.snapshots
		r.ElasticIPs += res.eips
		r.RDSInstances += res.rds
		r.LoadBalancers += res.lbs
		r.EKSClusters += res.eks
		r.LambdaCount += res.lambdas
		if res.hasEC2 {
			activeRegions++
		}
	}
	r.ActiveRegions = activeRegions

	log.Printf("[resource_collector/aws] EC2=%d running=%d RDS=%d LBs=%d EKS=%d Lambda=%d S3=%d",
		r.EC2Instances, r.RunningEC2, r.RDSInstances, r.LoadBalancers, r.EKSClusters, r.LambdaCount, r.S3Buckets)
	return r
}

// ── Azure ─────────────────────────────────────────────────────────────────────

func CollectAzureResources(ctx context.Context, creds map[string]string) AzureResourceCounts {
	r := AzureResourceCounts{}

	cred, err := azidentity.NewClientSecretCredential(
		creds["tenant_id"], creds["client_id"], creds["client_secret"], nil,
	)
	if err != nil {
		log.Printf("[resource_collector/azure] credential error: %v", err)
		return r
	}
	sid := creds["subscription_id"]

	// VMs
	vmC, _ := armcompute.NewVirtualMachinesClient(sid, cred, nil)
	for p := vmC.NewListAllPager(nil); p.More(); {
		pg, err := p.NextPage(ctx)
		if err != nil {
			break
		}
		r.VMs += len(pg.Value)
	}

	// Managed Disks
	diskC, _ := armcompute.NewDisksClient(sid, cred, nil)
	for p := diskC.NewListPager(nil); p.More(); {
		pg, err := p.NextPage(ctx)
		if err != nil {
			break
		}
		r.ManagedDisks += len(pg.Value)
	}

	// Snapshots
	snapC, _ := armcompute.NewSnapshotsClient(sid, cred, nil)
	for p := snapC.NewListPager(nil); p.More(); {
		pg, err := p.NextPage(ctx)
		if err != nil {
			break
		}
		r.Snapshots += len(pg.Value)
	}

	// Public IPs
	ipC, _ := armnetwork.NewPublicIPAddressesClient(sid, cred, nil)
	for p := ipC.NewListAllPager(nil); p.More(); {
		pg, err := p.NextPage(ctx)
		if err != nil {
			break
		}
		r.PublicIPs += len(pg.Value)
	}

	// SQL Databases (count individual DBs across all servers)
	sqlC, _ := armsql.NewServersClient(sid, cred, nil)
	for p := sqlC.NewListPager(nil); p.More(); {
		pg, err := p.NextPage(ctx)
		if err != nil {
			break
		}
		for _, srv := range pg.Value {
			if srv.Name == nil || srv.ID == nil {
				continue
			}
			rg := rgFromID(*srv.ID)
			dbC, _ := armsql.NewDatabasesClient(sid, cred, nil)
			for dp := dbC.NewListByServerPager(rg, *srv.Name, nil); dp.More(); {
				dbPg, err := dp.NextPage(ctx)
				if err != nil {
					break
				}
				r.Databases += len(dbPg.Value)
			}
		}
	}

	// AKS Clusters
	aksC, _ := armcontainerservice.NewManagedClustersClient(sid, cred, nil)
	for p := aksC.NewListPager(nil); p.More(); {
		pg, err := p.NextPage(ctx)
		if err != nil {
			break
		}
		r.AKSClusters += len(pg.Value)
	}

	// App Services
	webC, _ := armappservice.NewWebAppsClient(sid, cred, nil)
	for p := webC.NewListPager(nil); p.More(); {
		pg, err := p.NextPage(ctx)
		if err != nil {
			break
		}
		r.AppServices += len(pg.Value)
	}

	// Storage Accounts
	storC, _ := armstorage.NewAccountsClient(sid, cred, nil)
	for p := storC.NewListPager(nil); p.More(); {
		pg, err := p.NextPage(ctx)
		if err != nil {
			break
		}
		r.StorageAccounts += len(pg.Value)
	}

	log.Printf("[resource_collector/azure] VMs=%d Disks=%d AKS=%d SQL=%d AppSvc=%d Storage=%d",
		r.VMs, r.ManagedDisks, r.AKSClusters, r.Databases, r.AppServices, r.StorageAccounts)
	return r
}

// ── GCP ───────────────────────────────────────────────────────────────────────

func CollectGCPResources(ctx context.Context, creds map[string]string) GCPResourceCounts {
	r := GCPResourceCounts{}

	projectID := creds["project_id"]
	saKey := creds["service_account_key"]

	var opts []option.ClientOption
	if saKey != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(saKey)))
	} else if cf := creds["credentials_file"]; cf != "" {
		opts = append(opts, option.WithCredentialsFile(cf))
	}

	// VM Instances
	if instC, err := compute_gcp.NewInstancesRESTClient(ctx, opts...); err == nil {
		defer instC.Close()
		it := instC.AggregatedList(ctx, &computepb.AggregatedListInstancesRequest{Project: projectID})
		for {
			pair, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				break
			}
			r.VMInstances += len(pair.Value.Instances)
		}
	}

	// Disks
	if diskC, err := compute_gcp.NewDisksRESTClient(ctx, opts...); err == nil {
		defer diskC.Close()
		it := diskC.AggregatedList(ctx, &computepb.AggregatedListDisksRequest{Project: projectID})
		for {
			pair, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				break
			}
			r.Disks += len(pair.Value.Disks)
		}
	}

	// Snapshots
	if snapC, err := compute_gcp.NewSnapshotsRESTClient(ctx, opts...); err == nil {
		defer snapC.Close()
		it := snapC.List(ctx, &computepb.ListSnapshotsRequest{Project: projectID})
		for {
			if _, err := it.Next(); err != nil {
				break
			}
			r.Snapshots++
		}
	}

	// GKE Clusters
	if gkeC, err := container.NewClusterManagerClient(ctx, opts...); err == nil {
		defer gkeC.Close()
		if resp, err := gkeC.ListClusters(ctx, &containerpb.ListClustersRequest{
			Parent: fmt.Sprintf("projects/%s/locations/-", projectID),
		}); err == nil {
			r.GKEClusters = len(resp.Clusters)
		}
	}

	// Cloud SQL
	if sqlSvc, err := sqladmin.NewService(ctx, opts...); err == nil {
		if resp, err := sqlSvc.Instances.List(projectID).Do(); err == nil {
			r.SQLInstances = len(resp.Items)
		}
	}

	// GCS Buckets
	if gcsC, err := gcpstorage.NewClient(ctx, opts...); err == nil {
		defer gcsC.Close()
		it := gcsC.Buckets(ctx, projectID)
		for {
			if _, err := it.Next(); err != nil {
				break
			}
			r.GCSBuckets++
		}
	}

	// Cloud Functions
	if fnSvc, err := cloudfunctions.NewService(ctx, opts...); err == nil {
		if resp, err := fnSvc.Projects.Locations.Functions.List(
			fmt.Sprintf("projects/%s/locations/-", projectID),
		).Do(); err == nil {
			r.CloudFunctions = len(resp.Functions)
		}
	}

	log.Printf("[resource_collector/gcp] VMs=%d Disks=%d GKE=%d SQL=%d GCS=%d Functions=%d",
		r.VMInstances, r.Disks, r.GKEClusters, r.SQLInstances, r.GCSBuckets, r.CloudFunctions)
	return r
}

// ── buildTilesFromLiveCounts ──────────────────────────────────────────────────
// Builds report tiles using LIVE resource counts (not billing data).

func buildTilesFromLiveCounts(provider string, creds map[string]string) []reportTile {
	ctx := context.Background()

	switch provider {
	case "aws":
		c := CollectAWSResources(ctx, creds)
		return []reportTile{
			{Icon: "💻", Label: "EC2", Color: "#FF9900", Count: c.EC2Instances},
			{Icon: "✅", Label: "Running", Color: "#22C55E", Count: c.RunningEC2},
			{Icon: "💿", Label: "EBS GB", Color: "#3F8624", Count: int(c.EBSStorageGB)},
			{Icon: "🗃️", Label: "RDS", Color: "#8B5CF6", Count: c.RDSInstances},
			{Icon: "⚖️", Label: "LBs", Color: "#6366F1", Count: c.LoadBalancers},
			{Icon: "⚙️", Label: "EKS", Color: "#06B6D4", Count: c.EKSClusters},
			{Icon: "🪣", Label: "S3", Color: "#F59E0B", Count: c.S3Buckets},
			{Icon: "λ", Label: "Lambda", Color: "#EC4899", Count: c.LambdaCount},
			{Icon: "🌐", Label: "EIPs", Color: "#64748B", Count: c.ElasticIPs},
			{Icon: "📷", Label: "Snapshots", Color: "#6B7280", Count: c.Snapshots},
		}
	case "azure":
		c := CollectAzureResources(ctx, creds)
		return []reportTile{
			{Icon: "💻", Label: "VMs", Color: "#0078D4", Count: c.VMs},
			{Icon: "💾", Label: "Disks", Color: "#107C10", Count: c.ManagedDisks},
			{Icon: "📸", Label: "Snapshots", Color: "#6B7280", Count: c.Snapshots},
			{Icon: "🌐", Label: "Public IPs", Color: "#00B4D8", Count: c.PublicIPs},
			{Icon: "☸️", Label: "AKS", Color: "#06B6D4", Count: c.AKSClusters},
			{Icon: "🗃️", Label: "SQL DBs", Color: "#8B5CF6", Count: c.Databases},
			{Icon: "🚀", Label: "App Svc", Color: "#EC4899", Count: c.AppServices},
			{Icon: "🗄️", Label: "Storage", Color: "#3F8624", Count: c.StorageAccounts},
		}
	case "gcp":
		c := CollectGCPResources(ctx, creds)
		return []reportTile{
			{Icon: "💻", Label: "VMs", Color: "#4285F4", Count: c.VMInstances},
			{Icon: "💿", Label: "Disks", Color: "#34A853", Count: c.Disks},
			{Icon: "📷", Label: "Snapshots", Color: "#6B7280", Count: c.Snapshots},
			{Icon: "⚙️", Label: "GKE", Color: "#06B6D4", Count: c.GKEClusters},
			{Icon: "🗃️", Label: "Cloud SQL", Color: "#8B5CF6", Count: c.SQLInstances},
			{Icon: "🪣", Label: "GCS", Color: "#F59E0B", Count: c.GCSBuckets},
			{Icon: "λ", Label: "Functions", Color: "#EC4899", Count: c.CloudFunctions},
		}
	}
	return nil
}

// rgFromID extracts the resource group name from an Azure resource ID.
// e.g. /subscriptions/.../resourceGroups/myRG/providers/... → "myRG"
func rgFromID(id string) string {
	parts := strings.Split(id, "/")
	for i, p := range parts {
		if strings.EqualFold(p, "resourceGroups") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
