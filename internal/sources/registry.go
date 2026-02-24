package sources

type Source struct {
	URL      string
	Category string
}

func DefaultRegistry() []Source {
	// Keep entries deduplicated and stable because this list is user-visible.
	return []Source{
		{URL: "https://raw.githubusercontent.com/barry-far/V2ray-Configs/main/All_Configs_Sub.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/mfuu/v2ray/master/v2ray", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/Pawdroid/Free-servers/main/sub", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/aiboboxx/v2rayfree/main/v2", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/ripaojiedian/freenode/main/sub", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/peasoft/NoMoreWalls/master/list.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/mahdibland/V2RayAggregator/master/sub/sub_merge.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/Leon406/SubCrawler/main/sub/share/all3", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/ts-sf/fly/main/v2", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/freefq/free/master/v2", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/ssrsub/ssr/master/v2ray", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/Alvin9999/pac2/master/v2ray/1/config.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/Alvin9999/pac2/master/v2ray/2/config.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/Alvin9999/pac2/master/v2ray/3/config.txt", Category: "github-raw"},
		{URL: "https://sub.pmsub.me/base64", Category: "subscription"},
		{URL: "https://raw.githubusercontent.com/tbbatbb/Proxy/master/dist/v2ray.config.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/changfengoss/pub/main/data/2024_01_17/cvjOPc.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/ermaozi/get_subscribe/main/subscribe/v2ray.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/w1770946466/Auto_proxy/main/Long_term_subscription1.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/w1770946466/Auto_proxy/main/Long_term_subscription2.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/w1770946466/Auto_proxy/main/Long_term_subscription3.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/pojokan-id/v2ray-free/main/v2ray", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/yebekhe/v2rayfree/main/v2", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/freev2fly/freev2fly.github.io/gh-pages/_data/freeservers.json", Category: "github-raw-json"},
		{URL: "https://raw.githubusercontent.com/wzz1000/ClashR_for_Windows_Alpha/main/v2ray.config", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/ALX-003/V2RAY-XRAYS-2024/main/V2RAY-XVLESS-XVmess/V2RAY-VMESS-VLESS-TROJAN-2024.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/FoxiYao/freesub/main/v2ray", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/roshan9/v2ray/main/v2ray", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/anam2812/v2ray-servers/main/v2ray-servers.txt", Category: "github-raw"},
		{URL: "https://raw.githubusercontent.com/aliaraz/v2ray-config/main/config.txt", Category: "github-raw"},
	}
}

func DefaultURLs() []string {
	registry := DefaultRegistry()
	out := make([]string, 0, len(registry))
	for _, src := range registry {
		out = append(out, src.URL)
	}
	return out
}
