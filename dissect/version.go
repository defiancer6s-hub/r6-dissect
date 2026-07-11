package dissect

const (
	Y7S1        int = 6884476
	Y7S2        int = 7040830
	Y7S4        int = 7338571
	Y8S1        int = 7408213
	Y8S2        int = 7601998
	Y8S3        int = 7762708
	Y8S4        int = 7921866
	Y9S1        int = 8111697
	Y9S1Update3 int = 8211379
	Y9S2        int = 8303162
	Y9S3        int = 8506016
	Y9S4        int = 8673114
	Y10S1       int = 8825661
	Y10S1_1     int = 8863180
	Y10S1_2     int = 8882422
	Y10S1_3     int = 8908078
	Y10S2_1     int = 9034019
	Y10S2_1_1   int = 9058361
	Y10S2_2     int = 9077538
	Y10S2_3     int = 9098584
	Y10S2_4     int = 9124272
	Y10S2_5     int = 9158643
	Y10S3       int = 9199003
	Y10S3_1     int = 9211553
	Y11S1A3     int = 9679781
	// Y11S2 alpha changed the in-record field fingerprints for the player
	// DissectID and uiID (see readPlayer).
	Y11S2 int = 9734089
	// Y11S2A4 (alpha 4) reverted the DissectID and uiID field tags back to
	// the classic values, so the alpha-3 special-casing must be scoped to
	// [Y11S2, Y11S2A4) only.
	Y11S2A4 int = 9751808
)
